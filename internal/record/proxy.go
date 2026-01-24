package record

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/trace"
	"github.com/regrada-ai/regrada/internal/util"
)

type ProxyRecorder struct {
	cfg       *config.ProjectConfig
	store     trace.Store
	redactor  trace.Redactor
	session   *Session
	server    *http.Server
	done      chan struct{}
	stopAfter int
	mu        sync.Mutex
	count     int
	uploader  *Uploader
}

func NewProxyRecorder(cfg *config.ProjectConfig, store trace.Store, redactor trace.Redactor, session *Session) *ProxyRecorder {
	return &ProxyRecorder{
		cfg:      cfg,
		store:    store,
		redactor: redactor,
		session:  session,
		done:     make(chan struct{}),
		uploader: NewUploader(cfg),
	}
}

func (r *ProxyRecorder) Start() error {
	if r.cfg == nil {
		return fmt.Errorf("config is required")
	}
	addr := r.cfg.Capture.Proxy.Listen
	if addr == "" {
		addr = "127.0.0.1:4141"
	}

	r.server = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(r.handle),
	}

	go func() {
		_ = r.server.ListenAndServe()
		close(r.done)
	}()
	return nil
}

func (r *ProxyRecorder) Stop() error {
	if r.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r.server.Shutdown(ctx)
}

func (r *ProxyRecorder) Done() <-chan struct{} {
	return r.done
}

func (r *ProxyRecorder) TraceCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func (r *ProxyRecorder) Session() *Session {
	return r.session
}

func (r *ProxyRecorder) SetStopAfter(n int) {
	r.stopAfter = n
}

func (r *ProxyRecorder) handle(w http.ResponseWriter, req *http.Request) {
	upstream, provider, err := r.resolveUpstream(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	fmt.Printf("[proxy] %s %s → %s\n", req.Method, req.URL.Path, provider)

	start := time.Now()
	requestBody, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(requestBody))

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ModifyResponse = func(resp *http.Response) error {
		responseBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(responseBody))

		traceRecord := r.buildTrace(provider, requestBody, responseBody, time.Since(start))
		if r.redactor != nil && r.cfg.Capture.Redact.Enabled != nil && *r.cfg.Capture.Redact.Enabled {
			traceRecord.RedactionApplied = r.redactor.Apply(&traceRecord)
		}
		_ = r.store.Append(traceRecord)
		if r.session != nil {
			r.session.AddTrace(traceRecord.TraceID)
		}
		r.incrementCount()

		// Upload to backend if enabled (async, non-blocking)
		if r.uploader != nil && r.uploader.IsEnabled() {
			go r.uploader.Upload(nil, traceRecord)
		}

		fmt.Printf("[proxy] ✓ captured trace %s (%dms)\n", traceRecord.TraceID, traceRecord.Metrics.LatencyMS)
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		http.Error(rw, err.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, req)
}

func (r *ProxyRecorder) incrementCount() {
	r.mu.Lock()
	r.count++
	reached := r.stopAfter > 0 && r.count >= r.stopAfter
	r.mu.Unlock()
	if reached {
		go func() {
			_ = r.Stop()
		}()
	}
}

func (r *ProxyRecorder) resolveUpstream(req *http.Request) (*url.URL, string, error) {
	up := r.cfg.Capture.Proxy.Upstream
	if up.AnthropicBaseURL != "" && (req.Header.Get("anthropic-version") != "" || req.Header.Get("x-api-key") != "") {
		return parseURL(up.AnthropicBaseURL, "anthropic")
	}
	if up.AzureOpenAIBaseURL != "" && strings.Contains(req.URL.Path, "/openai/") {
		return parseURL(up.AzureOpenAIBaseURL, "azure_openai")
	}
	if up.BedrockBaseURL != "" && strings.Contains(strings.ToLower(req.URL.Path), "bedrock") {
		return parseURL(up.BedrockBaseURL, "bedrock")
	}
	if up.OpenAIBaseURL != "" {
		return parseURL(up.OpenAIBaseURL, "openai")
	}
	return nil, "", fmt.Errorf("no upstream configured")
}

func parseURL(raw string, provider string) (*url.URL, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	return u, provider, nil
}

func (r *ProxyRecorder) buildTrace(provider string, requestBody, responseBody []byte, latency time.Duration) trace.Trace {
	messages, params, modelName := parseRequest(requestBody)
	assistantText := parseResponse(responseBody)

	record := trace.Trace{
		TraceID:   util.NewID(),
		Timestamp: time.Now().UTC(),
		Provider:  provider,
		Model:     modelName,
		Request: trace.TraceRequest{
			Messages: messages,
			Params:   params,
		},
		Response: trace.TraceResponse{
			AssistantText: assistantText,
			Raw:           responseBody,
		},
		Metrics: trace.TraceMetrics{
			LatencyMS: int(latency.Milliseconds()),
		},
	}

	if len(messages) == 0 && len(requestBody) > 0 {
		record.Request.Messages = []model.Message{{Role: "user", Content: string(requestBody)}}
	}

	return record
}

func parseRequest(body []byte) ([]model.Message, *model.SamplingParams, string) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, ""
	}

	var messages []model.Message
	if rawMessages, ok := payload["messages"].([]any); ok {
		for _, item := range rawMessages {
			msgMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role, _ := msgMap["role"].(string)
			content := ""
			switch v := msgMap["content"].(type) {
			case string:
				content = v
			case []any:
				content = extractContentBlocks(v)
			}
			if role != "" && content != "" {
				messages = append(messages, model.Message{Role: role, Content: content})
			}
		}
	}

	if len(messages) == 0 {
		if prompt, ok := payload["prompt"].(string); ok {
			messages = append(messages, model.Message{Role: "user", Content: prompt})
		}
	}

	params := &model.SamplingParams{}
	if v, ok := payload["temperature"].(float64); ok {
		params.Temperature = &v
	}
	if v, ok := payload["top_p"].(float64); ok {
		params.TopP = &v
	}
	if v, ok := payload["max_output_tokens"].(float64); ok {
		val := int(v)
		params.MaxOutputTokens = &val
	}
	if v, ok := payload["max_tokens"].(float64); ok {
		val := int(v)
		params.MaxOutputTokens = &val
	}
	if stop, ok := payload["stop"].([]any); ok {
		params.Stop = toStringSlice(stop)
	}

	modelName, _ := payload["model"].(string)

	if params.Temperature == nil && params.TopP == nil && params.MaxOutputTokens == nil && len(params.Stop) == 0 {
		params = nil
	}

	return messages, params, modelName
}

func parseResponse(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	if choices, ok := payload["choices"].([]any); ok && len(choices) > 0 {
		choice, ok := choices[0].(map[string]any)
		if ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
			if content, ok := choice["text"].(string); ok {
				return content
			}
		}
	}

	if content, ok := payload["content"].(string); ok {
		return content
	}

	if blocks, ok := payload["content"].([]any); ok {
		return extractContentBlocks(blocks)
	}

	return ""
}

func extractContentBlocks(blocks []any) string {
	var parts []string
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := blockMap["text"].(string); ok {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

func toStringSlice(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
