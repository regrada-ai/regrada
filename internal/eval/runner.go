package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/providers"
)

type EvalSettings struct {
	Runs     int
	Timeout  time.Duration
	Sampling *model.SamplingParams
}

type CaseResult struct {
	CaseID     string      `json:"case_id"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	Runs       []RunResult `json:"runs"`
	Aggregates Aggregates  `json:"aggregates"`
}

type RunResult struct {
	RunID      int             `json:"run_id"`
	Pass       bool            `json:"pass"`
	OutputText string          `json:"output_text,omitempty"`
	JSON       json.RawMessage `json:"json,omitempty"`
	Metrics    RunMetrics      `json:"metrics"`
	Error      string          `json:"error,omitempty"`
}

type RunMetrics struct {
	LatencyMS int  `json:"latency_ms"`
	Refused   bool `json:"refused"`
	JSONValid bool `json:"json_valid"`
}

type Aggregates struct {
	PassRate      float64 `json:"pass_rate"`
	LatencyP95MS  int     `json:"latency_p95_ms"`
	RefusalRate   float64 `json:"refusal_rate"`
	JSONValidRate float64 `json:"json_valid_rate"`
}

func RunCase(ctx context.Context, c cases.Case, settings EvalSettings, provider providers.Provider) (CaseResult, error) {
	runs := settings.Runs
	if c.Runs != nil {
		runs = *c.Runs
	}
	if runs <= 0 {
		runs = 1
	}

	results := make([]RunResult, 0, runs)
	for i := 0; i < runs; i++ {
		runCtx := ctx
		if settings.Timeout > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, settings.Timeout)
			defer cancel()
		}

		req := buildProviderRequest(c, settings)
		start := time.Now()
		resp, timings, err := provider.Execute(runCtx, req)
		if err != nil {
			results = append(results, RunResult{
				RunID: i + 1,
				Pass:  false,
				Error: err.Error(),
			})
			continue
		}
		latency := timings.Latency
		if latency == 0 {
			latency = time.Since(start)
		}

		pass, jsonValid, errMsg := applyAsserts(c.Assert, resp.AssistantText)
		metrics := RunMetrics{
			LatencyMS: int(latency.Milliseconds()),
			Refused:   isRefusal(resp.AssistantText),
			JSONValid: jsonValid,
		}
		results = append(results, RunResult{
			RunID:      i + 1,
			Pass:       pass,
			OutputText: resp.AssistantText,
			JSON:       resp.Raw,
			Metrics:    metrics,
			Error:      errMsg,
		})
	}

	aggregates := computeAggregates(results)
	return CaseResult{
		CaseID:     c.ID,
		Provider:   provider.Name(),
		Runs:       results,
		Aggregates: aggregates,
	}, nil
}

func buildProviderRequest(c cases.Case, settings EvalSettings) providers.ProviderRequest {
	req := providers.ProviderRequest{
		Messages: c.Request.Messages,
		Input:    c.Request.Input,
	}

	params := &model.SamplingParams{}
	if c.Request.Params != nil {
		params.Temperature = c.Request.Params.Temperature
		params.TopP = c.Request.Params.TopP
		params.MaxOutputTokens = c.Request.Params.MaxOutputTokens
		params.Stop = c.Request.Params.Stop
	} else if c.Sampling != nil {
		params.Temperature = c.Sampling.Temperature
		params.TopP = c.Sampling.TopP
		params.MaxOutputTokens = c.Sampling.MaxOutputTokens
	} else if settings.Sampling != nil {
		params = settings.Sampling
	}

	if params.Temperature != nil || params.TopP != nil || params.MaxOutputTokens != nil || len(params.Stop) > 0 {
		req.Params = params
	}

	return req
}

func applyAsserts(asserts *cases.CaseAssert, output string) (bool, bool, string) {
	if asserts == nil {
		return true, isJSON(output), ""
	}

	if asserts.Text != nil {
		if len(asserts.Text.Contains) > 0 {
			for _, phrase := range asserts.Text.Contains {
				if !strings.Contains(output, phrase) {
					return false, isJSON(output), fmt.Sprintf("missing phrase: %s", phrase)
				}
			}
		}
		if len(asserts.Text.NotContains) > 0 {
			for _, phrase := range asserts.Text.NotContains {
				if strings.Contains(output, phrase) {
					return false, isJSON(output), fmt.Sprintf("unexpected phrase: %s", phrase)
				}
			}
		}
		if len(asserts.Text.Regex) > 0 {
			for _, pattern := range asserts.Text.Regex {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return false, isJSON(output), fmt.Sprintf("invalid regex: %s", pattern)
				}
				if !re.MatchString(output) {
					return false, isJSON(output), fmt.Sprintf("regex not matched: %s", pattern)
				}
			}
		}
		if len(asserts.Text.NotRegex) > 0 {
			for _, pattern := range asserts.Text.NotRegex {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return false, isJSON(output), fmt.Sprintf("invalid regex: %s", pattern)
				}
				if re.MatchString(output) {
					return false, isJSON(output), fmt.Sprintf("regex matched: %s", pattern)
				}
			}
		}
		if asserts.Text.MaxChars != nil && len(output) > *asserts.Text.MaxChars {
			return false, isJSON(output), "max_chars exceeded"
		}
	}

	jsonValid := isJSON(output)
	if asserts.JSON != nil {
		if asserts.JSON.Valid != nil && *asserts.JSON.Valid && !jsonValid {
			return false, jsonValid, "invalid json"
		}
		if asserts.JSON.Schema != "" {
			return false, jsonValid, "json schema validation not implemented"
		}
		if len(asserts.JSON.Path) > 0 {
			return false, jsonValid, "json path validation not implemented"
		}
	}

	return true, jsonValid, ""
}

func isJSON(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	var tmp any
	return json.Unmarshal([]byte(output), &tmp) == nil
}

func isRefusal(output string) bool {
	lower := strings.ToLower(output)
	phrases := []string{
		"i can't",
		"i cannot",
		"i’m sorry",
		"i'm sorry",
		"cannot help",
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func computeAggregates(results []RunResult) Aggregates {
	if len(results) == 0 {
		return Aggregates{}
	}
	passCount := 0
	refusalCount := 0
	jsonValidCount := 0
	latencies := make([]int, 0, len(results))
	for _, r := range results {
		if r.Pass {
			passCount++
		}
		if r.Metrics.Refused {
			refusalCount++
		}
		if r.Metrics.JSONValid {
			jsonValidCount++
		}
		latencies = append(latencies, r.Metrics.LatencyMS)
	}
	sort.Ints(latencies)

	p95 := latencies[len(latencies)-1]
	if len(latencies) > 1 {
		idx := int(float64(len(latencies)-1) * 0.95)
		p95 = latencies[idx]
	}

	total := float64(len(results))
	return Aggregates{
		PassRate:      float64(passCount) / total,
		LatencyP95MS:  p95,
		RefusalRate:   float64(refusalCount) / total,
		JSONValidRate: float64(jsonValidCount) / total,
	}
}
