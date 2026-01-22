package trace

import (
	"encoding/json"
	"time"

	"github.com/matias/regrada/internal/model"
)

type Trace struct {
	TraceID          string        `json:"trace_id"`
	Timestamp        time.Time     `json:"timestamp"`
	Provider         string        `json:"provider"`
	Model            string        `json:"model"`
	Environment      string        `json:"environment,omitempty"`
	GitSHA           string        `json:"git_sha,omitempty"`
	Request          TraceRequest  `json:"request"`
	Response         TraceResponse `json:"response"`
	Metrics          TraceMetrics  `json:"metrics"`
	RedactionApplied []string      `json:"redaction_applied,omitempty"`
}

type TraceRequest struct {
	Messages []model.Message       `json:"messages,omitempty"`
	Params   *model.SamplingParams `json:"params,omitempty"`
}

type TraceResponse struct {
	AssistantText string           `json:"assistant_text,omitempty"`
	ToolCalls     []model.ToolCall `json:"tool_calls,omitempty"`
	Raw           json.RawMessage  `json:"raw,omitempty"`
}

type TraceMetrics struct {
	LatencyMS int `json:"latency_ms,omitempty"`
	TokensIn  int `json:"tokens_in,omitempty"`
	TokensOut int `json:"tokens_out,omitempty"`
}
