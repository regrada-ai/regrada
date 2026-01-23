package cases

import (
	"fmt"

	"github.com/regrada-ai/regrada/internal/model"
	"gopkg.in/yaml.v3"
)

type Case struct {
	ID       string        `yaml:"id"`
	Tags     []string      `yaml:"tags,omitempty"`
	Request  CaseRequest   `yaml:"request"`
	Assert   *CaseAssert   `yaml:"assert,omitempty"`
	Runs     *int          `yaml:"runs,omitempty"`
	Sampling *CaseSampling `yaml:"sampling,omitempty"`
}

type CaseRequest struct {
	Messages []model.Message `yaml:"messages,omitempty"`
	Input    map[string]any  `yaml:"input,omitempty"`
	Params   *CaseParams     `yaml:"params,omitempty"`
}

type CaseParams struct {
	Temperature     *float64 `yaml:"temperature,omitempty"`
	TopP            *float64 `yaml:"top_p,omitempty"`
	MaxOutputTokens *int     `yaml:"max_output_tokens,omitempty"`
	Stop            []string `yaml:"stop,omitempty"`
}

type CaseSampling struct {
	Temperature     *float64 `yaml:"temperature,omitempty"`
	TopP            *float64 `yaml:"top_p,omitempty"`
	MaxOutputTokens *int     `yaml:"max_output_tokens,omitempty"`
}

type CaseAssert struct {
	Text    *TextAssert   `yaml:"text,omitempty"`
	JSON    *JSONAssert   `yaml:"json,omitempty"`
	Metrics *MetricAssert `yaml:"metrics,omitempty"`
}

type TextAssert struct {
	Contains    []string `yaml:"contains,omitempty"`
	NotContains []string `yaml:"not_contains,omitempty"`
	Regex       []string `yaml:"regex,omitempty"`
	NotRegex    []string `yaml:"not_regex,omitempty"`
	MaxChars    *int     `yaml:"max_chars,omitempty"`
}

type JSONAssert struct {
	Valid  *bool                          `yaml:"valid,omitempty"`
	Schema string                         `yaml:"schema,omitempty"`
	Path   map[string]JSONPathExpectation `yaml:"path,omitempty"`
}

type JSONPathExpectation struct {
	Eq  any      `yaml:"eq,omitempty"`
	Gte *float64 `yaml:"gte,omitempty"`
	Lte *float64 `yaml:"lte,omitempty"`
	In  []any    `yaml:"in,omitempty"`
}

type MetricAssert struct {
	LatencyP95MS *IntComparator   `yaml:"latency_p95_ms,omitempty"`
	RefusalRate  *FloatComparator `yaml:"refusal_rate,omitempty"`
}

type IntComparator struct {
	LTE *int `yaml:"lte,omitempty"`
}

type FloatComparator struct {
	LTE *float64 `yaml:"lte,omitempty"`
}

func ExampleCase() Case {
	maxChars := 120
	return Case{
		ID:   "example.greeting",
		Tags: []string{"example"},
		Request: CaseRequest{
			Messages: []model.Message{
				{Role: "system", Content: "You are a concise assistant."},
				{Role: "user", Content: "Say hello and ask for a name."},
			},
			Params: &CaseParams{Temperature: floatPtr(0.2), TopP: floatPtr(1.0)},
		},
		Assert: &CaseAssert{
			Text: &TextAssert{
				Contains: []string{"hello"},
				MaxChars: &maxChars,
			},
		},
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func (j JSONPathExpectation) Validate() error {
	if j.Eq != nil || j.Gte != nil || j.Lte != nil || len(j.In) > 0 {
		return nil
	}
	return fmt.Errorf("json.path expectation must include eq, gte, lte, or in")
}

func (j *JSONPathExpectation) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		var v any
		if err := value.Decode(&v); err != nil {
			return err
		}
		j.Eq = v
		return nil
	}

	var raw map[string]*yaml.Node
	if err := value.Decode(&raw); err != nil {
		return err
	}

	for key := range raw {
		switch key {
		case "eq", "gte", "lte", "in":
		default:
			return fmt.Errorf("unknown json.path comparator %q", key)
		}
	}

	if node, ok := raw["eq"]; ok {
		var v any
		if err := node.Decode(&v); err != nil {
			return err
		}
		j.Eq = v
	}
	if node, ok := raw["gte"]; ok {
		var v float64
		if err := node.Decode(&v); err != nil {
			return err
		}
		j.Gte = &v
	}
	if node, ok := raw["lte"]; ok {
		var v float64
		if err := node.Decode(&v); err != nil {
			return err
		}
		j.Lte = &v
	}
	if node, ok := raw["in"]; ok {
		var v []any
		if err := node.Decode(&v); err != nil {
			return err
		}
		j.In = v
	}

	return nil
}
