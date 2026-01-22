package providers

import (
	"context"
	"encoding/json"
	"time"
)

type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (p *MockProvider) Name() string {
	return "mock"
}

func (p *MockProvider) Execute(ctx context.Context, req ProviderRequest) (ProviderResponse, Timings, error) {
	response := ProviderResponse{
		AssistantText: "mock response",
		Raw:           json.RawMessage(`{"message":"mock response"}`),
	}
	return response, Timings{Latency: 10 * time.Millisecond}, nil
}
