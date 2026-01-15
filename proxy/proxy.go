package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/matias/regrada/config"
	"github.com/matias/regrada/trace"
)

// LLMProxy intercepts and records LLM API calls.
type LLMProxy struct {
	listener   net.Listener
	server     *http.Server
	traces     []trace.LLMTrace
	mu         sync.Mutex
	config     *config.RegradaConfig
	providers  map[string]*url.URL
	httpClient *http.Client
}

// New creates a new LLM proxy server.
// It listens on a random port on localhost and forwards requests to the configured LLM provider.
func New(cfg *config.RegradaConfig) (*LLMProxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}

	proxy := &LLMProxy{
		listener:  listener,
		traces:    []trace.LLMTrace{},
		config:    cfg,
		providers: make(map[string]*url.URL),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
				MaxIdleConns:    100,
				IdleConnTimeout: 90 * time.Second,
			},
		},
	}

	// Set up provider URL based on config
	var targetURL *url.URL
	switch cfg.Provider.Type {
	case "openai":
		targetURL, _ = url.Parse("https://api.openai.com")
	case "anthropic":
		targetURL, _ = url.Parse("https://api.anthropic.com")
	case "azure", "azure-openai":
		if cfg.Provider.BaseURL == "" {
			return nil, fmt.Errorf("Azure provider requires base_url in config")
		}
		targetURL, err = url.Parse(cfg.Provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Azure base_url: %w", err)
		}
	case "custom":
		if cfg.Provider.BaseURL == "" {
			return nil, fmt.Errorf("Custom provider requires base_url in config")
		}
		targetURL, err = url.Parse(cfg.Provider.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid custom base_url: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Provider.Type)
	}

	proxy.providers[cfg.Provider.Type] = targetURL

	mux := http.NewServeMux()
	mux.HandleFunc("/", proxy.handleRequest)

	proxy.server = &http.Server{
		Handler: mux,
	}

	go proxy.server.Serve(listener)

	return proxy, nil
}

// Address returns the address the proxy is listening on.
func (p *LLMProxy) Address() string {
	return p.listener.Addr().String()
}

// Traces returns a copy of all captured traces.
func (p *LLMProxy) Traces() []trace.LLMTrace {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]trace.LLMTrace{}, p.traces...)
}

// Shutdown gracefully shuts down the proxy server.
func (p *LLMProxy) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.server.Shutdown(ctx)
}

// handleRequest is the main proxy handler that intercepts, forwards, and records LLM API calls.
func (p *LLMProxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Use the configured provider type
	targetProvider := p.config.Provider.Type
	targetURL, ok := p.providers[targetProvider]
	if !ok {
		http.Error(w, fmt.Sprintf("Provider %s not configured", targetProvider), http.StatusBadGateway)
		return
	}

	// Read request body
	requestBody, err := p.readRequestBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create and execute proxy request
	proxyReq, err := p.createProxyRequest(r, targetURL, requestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, responseBody, err := p.executeProxyRequest(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(startTime)

	// Record trace
	tr := p.createTrace(targetProvider, r, requestBody, resp, responseBody, latency)
	p.mu.Lock()
	p.traces = append(p.traces, tr)
	p.mu.Unlock()

	// Write response to client
	p.writeResponse(w, resp, responseBody)
}

// readRequestBody reads and buffers the request body.
func (p *LLMProxy) readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	// Restore the body for further reading
	r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return requestBody, nil
}

// createProxyRequest creates a new HTTP request to forward to the LLM provider.
func (p *LLMProxy) createProxyRequest(r *http.Request, targetURL *url.URL, requestBody []byte) (*http.Request, error) {
	proxyURL := *targetURL
	proxyURL.Path = r.URL.Path
	proxyURL.RawQuery = r.URL.RawQuery

	proxyReq, err := http.NewRequest(r.Method, proxyURL.String(), bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	// Copy headers, removing proxy-specific ones
	for key, values := range r.Header {
		if strings.HasPrefix(key, "X-Regrada-") {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	return proxyReq, nil
}

// executeProxyRequest executes the proxy request and reads the response.
func (p *LLMProxy) executeProxyRequest(proxyReq *http.Request) (*http.Response, []byte, error) {
	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		return nil, nil, err
	}

	// Read response body, handling gzip encoding
	var responseBody []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err == nil {
			responseBody, _ = io.ReadAll(gzReader)
			gzReader.Close()
		}
	} else {
		responseBody, _ = io.ReadAll(resp.Body)
	}

	return resp, responseBody, nil
}

// writeResponse writes the proxied response back to the client.
func (p *LLMProxy) writeResponse(w http.ResponseWriter, resp *http.Response, responseBody []byte) {
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Remove Content-Encoding since we've already decompressed
	w.Header().Del("Content-Encoding")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))

	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)
}

// createTrace creates a trace record from the request and response.
func (p *LLMProxy) createTrace(provider string, req *http.Request, reqBody []byte, resp *http.Response, respBody []byte, latency time.Duration) trace.LLMTrace {
	tr := trace.LLMTrace{
		ID:        generateTraceID(),
		Timestamp: time.Now(),
		Provider:  provider,
		Endpoint:  req.URL.Path,
		Latency:   latency / time.Millisecond,
		Request: trace.TraceRequest{
			Method:  req.Method,
			Path:    req.URL.Path,
			Headers: flattenHeaders(req.Header),
			Body:    sanitizeBody(reqBody),
		},
		Response: trace.TraceResponse{
			StatusCode: resp.StatusCode,
			Headers:    flattenHeaders(resp.Header),
			Body:       sanitizeBody(respBody),
		},
	}

	// Extract model and tokens from request/response
	tr.Model, tr.TokensIn, tr.TokensOut, tr.ToolCalls = parseAPIDetails(provider, reqBody, respBody)

	return tr
}

// parseAPIDetails extracts provider-specific details from request and response bodies.
func parseAPIDetails(provider string, reqBody, respBody []byte) (model string, tokensIn, tokensOut int, toolCalls []trace.ToolCall) {
	var reqData map[string]interface{}
	var respData map[string]interface{}

	json.Unmarshal(reqBody, &reqData)
	json.Unmarshal(respBody, &respData)

	// Extract model from request
	if m, ok := reqData["model"].(string); ok {
		model = m
	}

	// Provider-specific parsing
	switch provider {
	case "openai":
		if usage, ok := respData["usage"].(map[string]interface{}); ok {
			if pt, ok := usage["prompt_tokens"].(float64); ok {
				tokensIn = int(pt)
			}
			if ct, ok := usage["completion_tokens"].(float64); ok {
				tokensOut = int(ct)
			}
		}
		// Extract tool calls
		if choices, ok := respData["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := choice["message"].(map[string]interface{}); ok {
					if tcs, ok := msg["tool_calls"].([]interface{}); ok {
						for _, tc := range tcs {
							if tcMap, ok := tc.(map[string]interface{}); ok {
								toolCall := trace.ToolCall{
									ID: getString(tcMap, "id"),
								}
								if fn, ok := tcMap["function"].(map[string]interface{}); ok {
									toolCall.Name = getString(fn, "name")
									if args, ok := fn["arguments"].(string); ok {
										toolCall.Args = json.RawMessage(args)
									}
								}
								toolCalls = append(toolCalls, toolCall)
							}
						}
					}
				}
			}
		}

	case "anthropic":
		if usage, ok := respData["usage"].(map[string]interface{}); ok {
			if it, ok := usage["input_tokens"].(float64); ok {
				tokensIn = int(it)
			}
			if ot, ok := usage["output_tokens"].(float64); ok {
				tokensOut = int(ot)
			}
		}
		// Extract tool use from Anthropic format
		if content, ok := respData["content"].([]interface{}); ok {
			for _, c := range content {
				if cMap, ok := c.(map[string]interface{}); ok {
					if cMap["type"] == "tool_use" {
						toolCall := trace.ToolCall{
							ID:   getString(cMap, "id"),
							Name: getString(cMap, "name"),
						}
						if input, ok := cMap["input"]; ok {
							if inputBytes, err := json.Marshal(input); err == nil {
								toolCall.Args = json.RawMessage(inputBytes)
							}
						}
						toolCalls = append(toolCalls, toolCall)
					}
				}
			}
		}

	case "custom":
		// Handle Ollama and other custom providers
		// Try Ollama format first
		if msg, ok := respData["message"].(map[string]interface{}); ok {
			// Extract tool calls from Ollama format
			if tcs, ok := msg["tool_calls"].([]interface{}); ok {
				for _, tc := range tcs {
					if tcMap, ok := tc.(map[string]interface{}); ok {
						toolCall := trace.ToolCall{
							ID: getString(tcMap, "id"),
						}
						if fn, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Name = getString(fn, "name")
							if args, ok := fn["arguments"]; ok {
								if argsBytes, err := json.Marshal(args); err == nil {
									toolCall.Args = json.RawMessage(argsBytes)
								}
							}
						}
						toolCalls = append(toolCalls, toolCall)
					}
				}
			}
		}

		// Extract token counts for Ollama
		if pc, ok := respData["prompt_eval_count"].(float64); ok {
			tokensIn = int(pc)
		}
		if ec, ok := respData["eval_count"].(float64); ok {
			tokensOut = int(ec)
		}

		// Fallback: try OpenAI-compatible format for custom providers
		if len(toolCalls) == 0 {
			if choices, ok := respData["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if msg, ok := choice["message"].(map[string]interface{}); ok {
						if tcs, ok := msg["tool_calls"].([]interface{}); ok {
							for _, tc := range tcs {
								if tcMap, ok := tc.(map[string]interface{}); ok {
									toolCall := trace.ToolCall{
										ID: getString(tcMap, "id"),
									}
									if fn, ok := tcMap["function"].(map[string]interface{}); ok {
										toolCall.Name = getString(fn, "name")
										if args, ok := fn["arguments"]; ok {
											if argsStr, ok := args.(string); ok {
												toolCall.Args = json.RawMessage(argsStr)
											} else if argsBytes, err := json.Marshal(args); err == nil {
												toolCall.Args = json.RawMessage(argsBytes)
											}
										}
									}
									toolCalls = append(toolCalls, toolCall)
								}
							}
						}
					}
				}
			}
		}
	}

	return
}

// Helper functions

func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range h {
		// Skip sensitive headers
		lowerKey := strings.ToLower(key)
		if lowerKey == "authorization" || lowerKey == "x-api-key" || lowerKey == "api-key" {
			result[key] = "[REDACTED]"
			continue
		}
		result[key] = strings.Join(values, ", ")
	}
	return result
}

func sanitizeBody(body []byte) json.RawMessage {
	if len(body) == 0 {
		return nil
	}

	// Try to parse as JSON to validate
	var js interface{}
	if json.Unmarshal(body, &js) != nil {
		// Not valid JSON, return as quoted string
		quoted, _ := json.Marshal(string(body))
		return json.RawMessage(quoted)
	}

	return json.RawMessage(body)
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
