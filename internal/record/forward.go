package record

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/regrada-ai/regrada/internal/ca"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/trace"
	"github.com/regrada-ai/regrada/internal/util"
)

// ForwardProxyRecorder implements a forward proxy with HTTPS MITM
type ForwardProxyRecorder struct {
	cfg      *config.ProjectConfig
	store    trace.Store
	redactor trace.Redactor
	session  *Session
	proxy    *goproxy.ProxyHttpServer
	server   *http.Server
	ca       *ca.CA
	matcher  *hostMatcher
	count    int
}

// NewForwardProxyRecorder creates a forward proxy recorder with MITM
func NewForwardProxyRecorder(cfg *config.ProjectConfig, store trace.Store, redactor trace.Redactor, session *Session) (*ForwardProxyRecorder, error) {
	// Load CA certificate
	caPath := cfg.Capture.Proxy.CAPath
	var caObj *ca.CA
	var err error

	if ca.Exists(caPath) {
		caObj, err = ca.Load(caPath)
		if err != nil {
			return nil, fmt.Errorf("load CA: %w", err)
		}
	} else {
		return nil, fmt.Errorf("CA not found at %s. Run 'regrada ca init' first", caPath)
	}

	matcher := newHostMatcher(cfg.Capture.Proxy.AllowHosts)

	fpr := &ForwardProxyRecorder{
		cfg:      cfg,
		store:    store,
		redactor: redactor,
		session:  session,
		ca:       caObj,
		matcher:  matcher,
	}

	fpr.setupProxy()
	return fpr, nil
}

// setupProxy configures goproxy
func (r *ForwardProxyRecorder) setupProxy() {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false

	// Set CA certificate for MITM
	goproxyCa := tls.Certificate{
		Certificate: [][]byte{r.ca.Cert().Raw},
		PrivateKey:  r.ca.Key(),
		Leaf:        r.ca.Cert(),
	}
	goproxy.GoproxyCa = goproxyCa

	// CONNECT handler: MITM only allowlisted hosts
	proxy.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if r.matcher.isAllowed(host) {
			return goproxy.MitmConnect, host
		}
		return goproxy.OkConnect, host
	}))

	// Request hook
	proxy.OnRequest().DoFunc(r.handleRequest)

	// Response hook
	proxy.OnResponse().DoFunc(r.handleResponse)

	// Transport for upstream
	proxy.Tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	r.proxy = proxy
}

// handleRequest captures request data
func (r *ForwardProxyRecorder) handleRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	host := req.URL.Hostname()
	if host == "" {
		host = req.Host
	}

	if !r.matcher.isAllowed(host) {
		if r.cfg.Capture.Proxy.Debug {
			fmt.Printf("[DEBUG] Skipping request to %s (not in allowed hosts: %v)\n", host, r.cfg.Capture.Proxy.AllowHosts)
		}
		return req, nil
	}

	if r.cfg.Capture.Proxy.Debug {
		fmt.Printf("[DEBUG] Capturing request to %s %s\n", req.Method, req.URL.String())
	}

	// Capture context
	capture := &captureContext{
		traceID:    util.NewID(),
		startTime:  time.Now(),
		provider:   r.matcher.provider(host),
		host:       host,
		method:     req.Method,
		scheme:     req.URL.Scheme,
		path:       req.URL.Path,
		reqHeaders: cloneHeaders(req.Header),
	}

	// Read and restore body
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		capture.requestBody = body
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	ctx.UserData = capture
	return req, nil
}

// handleResponse captures response and writes trace
func (r *ForwardProxyRecorder) handleResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	capture, ok := ctx.UserData.(*captureContext)
	if !ok || capture == nil {
		return resp
	}

	duration := time.Since(capture.startTime)
	capture.statusCode = resp.StatusCode
	capture.respHeaders = cloneHeaders(resp.Header)

	// Read and restore response body
	if resp.Body != nil {
		body, _ := readBodyWithGzip(resp.Body, resp.Header)
		capture.responseBody = body
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Build trace
	tr := r.buildTraceFromCapture(capture, duration)

	if r.redactor != nil && r.cfg.Capture.Redact.Enabled != nil && *r.cfg.Capture.Redact.Enabled {
		tr.RedactionApplied = r.redactor.Apply(&tr)
	}

	_ = r.store.Append(tr)
	if r.session != nil {
		r.session.AddTrace(tr.TraceID)
	}
	r.count++

	if r.cfg.Capture.Proxy.Debug {
		fmt.Printf("[DEBUG] Trace recorded: %s (provider: %s, model: %s, latency: %dms)\n",
			tr.TraceID, tr.Provider, tr.Model, tr.Metrics.LatencyMS)
	}

	return resp
}

// Start starts the forward proxy server
func (r *ForwardProxyRecorder) Start() error {
	addr := r.cfg.Capture.Proxy.Listen
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	r.server = &http.Server{
		Addr:    addr,
		Handler: r.proxy,
	}

	go func() {
		_ = r.server.ListenAndServe()
	}()

	return nil
}

// Stop stops the proxy
func (r *ForwardProxyRecorder) Stop() error {
	if r.server == nil {
		return nil
	}
	return r.server.Close()
}

// TraceCount returns number of traces
func (r *ForwardProxyRecorder) TraceCount() int {
	return r.count
}

// Session returns the session
func (r *ForwardProxyRecorder) Session() *Session {
	return r.session
}

// buildTraceFromCapture converts capture context to trace
func (r *ForwardProxyRecorder) buildTraceFromCapture(c *captureContext, duration time.Duration) trace.Trace {
	messages, params, modelName := parseRequest(c.requestBody)
	assistantText := parseResponse(c.responseBody)

	return trace.Trace{
		TraceID:   c.traceID,
		Timestamp: c.startTime.UTC(),
		Provider:  c.provider,
		Model:     modelName,
		Request: trace.TraceRequest{
			Messages: messages,
			Params:   params,
		},
		Response: trace.TraceResponse{
			AssistantText: assistantText,
			Raw:           c.responseBody,
		},
		Metrics: trace.TraceMetrics{
			LatencyMS: int(duration.Milliseconds()),
		},
	}
}

// Helper types and functions

type captureContext struct {
	traceID      string
	startTime    time.Time
	provider     string
	host         string
	method       string
	scheme       string
	path         string
	statusCode   int
	requestBody  []byte
	responseBody []byte
	reqHeaders   http.Header
	respHeaders  http.Header
}

type hostMatcher struct {
	allowHosts map[string]string
}

func newHostMatcher(hosts []string) *hostMatcher {
	m := &hostMatcher{allowHosts: make(map[string]string)}
	for _, host := range hosts {
		m.allowHosts[strings.ToLower(host)] = deriveProvider(host)
	}
	return m
}

func (m *hostMatcher) isAllowed(host string) bool {
	_, ok := m.allowHosts[strings.ToLower(host)]
	return ok
}

func (m *hostMatcher) provider(host string) string {
	return m.allowHosts[strings.ToLower(host)]
}

func deriveProvider(host string) string {
	host = strings.ToLower(host)
	if strings.Contains(host, "openai") {
		if strings.Contains(host, "azure") {
			return "azure_openai"
		}
		return "openai"
	}
	if strings.Contains(host, "anthropic") {
		return "anthropic"
	}
	if strings.Contains(host, "bedrock") || strings.Contains(host, "amazonaws") {
		return "bedrock"
	}
	parts := strings.Split(host, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func cloneHeaders(h http.Header) http.Header {
	clone := make(http.Header, len(h))
	for k, v := range h {
		clone[k] = append([]string(nil), v...)
	}
	return clone
}

func readBodyWithGzip(body io.ReadCloser, headers http.Header) ([]byte, error) {
	defer body.Close()
	var reader io.Reader = body
	if headers.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(body)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		reader = gr
	}
	return io.ReadAll(reader)
}

func hashValue(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:4])
}
