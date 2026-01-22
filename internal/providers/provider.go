package providers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/matias/regrada/internal/model"
)

type Provider interface {
	Name() string
	Execute(ctx context.Context, req ProviderRequest) (ProviderResponse, Timings, error)
}

type ProviderRequest struct {
	Messages []model.Message       `json:"messages,omitempty"`
	Input    map[string]any        `json:"input,omitempty"`
	Params   *model.SamplingParams `json:"params,omitempty"`
}

type ProviderResponse struct {
	AssistantText string          `json:"assistant_text,omitempty"`
	Raw           json.RawMessage `json:"raw,omitempty"`
}

type Timings struct {
	Latency time.Duration
}
