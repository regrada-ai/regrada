package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/model"
)

type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAIProvider(cfg *config.ProjectConfig) (*OpenAIProvider, error) {
	// API Key: env var > hardcoded value
	apiKey := ""
	if cfg.Providers.OpenAI.APIKeyEnv != "" {
		apiKey = strings.TrimSpace(os.Getenv(cfg.Providers.OpenAI.APIKeyEnv))
	}
	if apiKey == "" && cfg.Providers.OpenAI.APIKey != "" {
		apiKey = strings.TrimSpace(cfg.Providers.OpenAI.APIKey)
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing OpenAI API key (set providers.openai.api_key, api_key_env, or OPENAI_API_KEY)")
	}

	// Base URL: env var > hardcoded value > default
	baseURL := ""
	if cfg.Providers.OpenAI.BaseURLEnv != "" {
		baseURL = strings.TrimSpace(os.Getenv(cfg.Providers.OpenAI.BaseURLEnv))
	}
	if baseURL == "" && cfg.Providers.OpenAI.BaseURL != "" {
		baseURL = strings.TrimSpace(cfg.Providers.OpenAI.BaseURL)
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Model: config > env var
	modelName := strings.TrimSpace(cfg.Providers.OpenAI.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if modelName == "" {
		return nil, fmt.Errorf("openai model is required (set providers.openai.model or OPENAI_MODEL)")
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   modelName,
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Execute(ctx context.Context, req ProviderRequest) (ProviderResponse, Timings, error) {
	messages := req.Messages
	if len(messages) == 0 && req.Input != nil {
		inputBytes, err := json.Marshal(req.Input)
		if err != nil {
			return ProviderResponse{}, Timings{}, fmt.Errorf("marshal input: %w", err)
		}
		messages = []model.Message{{Role: "user", Content: string(inputBytes)}}
	}
	if len(messages) == 0 {
		return ProviderResponse{}, Timings{}, fmt.Errorf("no messages provided")
	}

	payload := map[string]any{
		"model":    p.model,
		"messages": messages,
	}

	if req.Params != nil {
		if req.Params.Temperature != nil {
			payload["temperature"] = *req.Params.Temperature
		}
		if req.Params.TopP != nil {
			payload["top_p"] = *req.Params.TopP
		}
		if req.Params.MaxOutputTokens != nil {
			payload["max_tokens"] = *req.Params.MaxOutputTokens
		}
		if len(req.Params.Stop) > 0 {
			payload["stop"] = req.Params.Stop
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ProviderResponse{}, Timings{}, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := openAIEndpoint(p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProviderResponse{}, Timings{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return ProviderResponse{}, Timings{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProviderResponse{}, Timings{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProviderResponse{}, Timings{}, fmt.Errorf("openai error %d: %s", resp.StatusCode, string(respBody))
	}

	text := parseOpenAIResponse(respBody)
	return ProviderResponse{
		AssistantText: text,
		Raw:           respBody,
	}, Timings{Latency: time.Since(start)}, nil
}

func openAIEndpoint(base string) string {
	trimmed := strings.TrimRight(base, "/")
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed + "/chat/completions"
	}
	return trimmed + "/v1/chat/completions"
}

func parseOpenAIResponse(body []byte) string {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if len(payload.Choices) == 0 {
		return ""
	}
	if payload.Choices[0].Message.Content != "" {
		return payload.Choices[0].Message.Content
	}
	return payload.Choices[0].Text
}
