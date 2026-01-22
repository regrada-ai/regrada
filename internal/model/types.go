package model

import "encoding/json"

type Message struct {
	Role    string `json:"role" yaml:"role"`
	Content string `json:"content" yaml:"content"`
}

type SamplingParams struct {
	Temperature     *float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty" yaml:"max_output_tokens,omitempty"`
	Stop            []string `json:"stop,omitempty" yaml:"stop,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Response  json.RawMessage `json:"response,omitempty"`
}
