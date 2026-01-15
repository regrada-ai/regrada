package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LLMTrace represents a captured LLM API call with request/response details.
// It includes metadata such as latency, token counts, and tool calls.
type LLMTrace struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Provider  string            `json:"provider"`
	Endpoint  string            `json:"endpoint"`
	Model     string            `json:"model,omitempty"`
	Request   TraceRequest      `json:"request"`
	Response  TraceResponse     `json:"response"`
	Latency   time.Duration     `json:"latency_ms"`
	ToolCalls []ToolCall        `json:"tool_calls,omitempty"`
	TokensIn  int               `json:"tokens_in,omitempty"`
	TokensOut int               `json:"tokens_out,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// TraceRequest contains the HTTP request details of an LLM API call.
type TraceRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// TraceResponse contains the HTTP response details of an LLM API call.
type TraceResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       json.RawMessage   `json:"body,omitempty"`
}

// ToolCall represents a function/tool invocation by the LLM.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Args     json.RawMessage `json:"arguments"`
	Response json.RawMessage `json:"response,omitempty"`
}

// TraceSession holds all traces from a single run.
type TraceSession struct {
	ID        string       `json:"id"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time"`
	Command   string       `json:"command"`
	Traces    []LLMTrace   `json:"traces"`
	Summary   TraceSummary `json:"summary"`
}

// TraceSummary aggregates statistics from all traces in a session.
type TraceSummary struct {
	TotalCalls     int            `json:"total_calls"`
	TotalTokensIn  int            `json:"total_tokens_in"`
	TotalTokensOut int            `json:"total_tokens_out"`
	TotalLatency   time.Duration  `json:"total_latency_ms"`
	ByProvider     map[string]int `json:"by_provider"`
	ByModel        map[string]int `json:"by_model"`
	ToolsCalled    []string       `json:"tools_called"`
}

// Comparison represents the difference between a current session and a baseline.
type Comparison struct {
	CallCountChanged bool                       `json:"CallCountChanged"`
	BaselineCount    int                        `json:"BaselineCount"`
	CurrentCount     int                        `json:"CurrentCount"`
	NewTools         []string                   `json:"NewTools"`
	RemovedTools     []string                   `json:"RemovedTools"`
	ModelChanges     map[string]ModelChange     `json:"ModelChanges"`
	TokenDiff        int                        `json:"TokenDiff"`
}

// ModelChange represents a change in model usage.
type ModelChange struct {
	Model         string `json:"Model"`
	BaselineCount int    `json:"BaselineCount"`
	CurrentCount  int    `json:"CurrentCount"`
	IsNew         bool   `json:"IsNew"`
}

// Save writes a trace session to a file in JSON format.
func Save(session *TraceSession, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load reads a trace session from a file.
func Load(path string) (*TraceSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session TraceSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// Compare analyzes the difference between a current session and a baseline.
// Returns nil if baseline doesn't exist or can't be loaded.
func Compare(current *TraceSession, baselinePath string) (*Comparison, error) {
	baseline, err := Load(baselinePath)
	if err != nil {
		return nil, err
	}

	comp := &Comparison{
		CallCountChanged: current.Summary.TotalCalls != baseline.Summary.TotalCalls,
		BaselineCount:    baseline.Summary.TotalCalls,
		CurrentCount:     current.Summary.TotalCalls,
		NewTools:         []string{},
		RemovedTools:     []string{},
		ModelChanges:     make(map[string]ModelChange),
	}

	// Compare tools called
	baselineTools := make(map[string]bool)
	for _, t := range baseline.Summary.ToolsCalled {
		baselineTools[t] = true
	}

	currentTools := make(map[string]bool)
	for _, t := range current.Summary.ToolsCalled {
		currentTools[t] = true
	}

	// Find new and removed tools
	for tool := range currentTools {
		if !baselineTools[tool] {
			comp.NewTools = append(comp.NewTools, tool)
		}
	}

	for tool := range baselineTools {
		if !currentTools[tool] {
			comp.RemovedTools = append(comp.RemovedTools, tool)
		}
	}

	// Compare models
	for model, count := range current.Summary.ByModel {
		if baselineCount, ok := baseline.Summary.ByModel[model]; ok {
			if count != baselineCount {
				comp.ModelChanges[model] = ModelChange{
					Model:         model,
					BaselineCount: baselineCount,
					CurrentCount:  count,
					IsNew:         false,
				}
			}
		} else {
			comp.ModelChanges[model] = ModelChange{
				Model:         model,
				BaselineCount: 0,
				CurrentCount:  count,
				IsNew:         true,
			}
		}
	}

	// Calculate token usage difference
	comp.TokenDiff = (current.Summary.TotalTokensIn + current.Summary.TotalTokensOut) -
		(baseline.Summary.TotalTokensIn + baseline.Summary.TotalTokensOut)

	return comp, nil
}

// CalculateSummary aggregates statistics from a list of traces.
func CalculateSummary(traces []LLMTrace) TraceSummary {
	summary := TraceSummary{
		TotalCalls:  len(traces),
		ByProvider:  make(map[string]int),
		ByModel:     make(map[string]int),
		ToolsCalled: []string{},
	}

	toolSet := make(map[string]bool)

	for _, t := range traces {
		summary.TotalTokensIn += t.TokensIn
		summary.TotalTokensOut += t.TokensOut
		summary.TotalLatency += t.Latency
		summary.ByProvider[t.Provider]++
		if t.Model != "" {
			summary.ByModel[t.Model]++
		}
		for _, tc := range t.ToolCalls {
			toolSet[tc.Name] = true
		}
	}

	for tool := range toolSet {
		summary.ToolsCalled = append(summary.ToolsCalled, tool)
	}

	return summary
}

// PrintSummary displays a trace session summary to stdout.
// This is a helper function for command-line output.
func PrintSummary(session *TraceSession) {
	summary := session.Summary
	duration := session.EndTime.Sub(session.StartTime).Round(time.Millisecond)

	fmt.Printf("✓ Captured %d LLM calls in %v\n", summary.TotalCalls, duration)

	if summary.TotalCalls == 0 {
		fmt.Println("  No LLM API calls detected")
		return
	}

	fmt.Println()
	fmt.Println("  Summary:")

	if len(summary.ByProvider) > 0 {
		fmt.Print("    Providers: ")
		first := true
		for provider, count := range summary.ByProvider {
			if !first {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%d)", provider, count)
			first = false
		}
		fmt.Println()
	}

	if len(summary.ByModel) > 0 {
		fmt.Print("    Models: ")
		first := true
		for model, count := range summary.ByModel {
			if !first {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%d)", model, count)
			first = false
		}
		fmt.Println()
	}

	if summary.TotalTokensIn > 0 || summary.TotalTokensOut > 0 {
		fmt.Printf("    Tokens: %d in / %d out\n", summary.TotalTokensIn, summary.TotalTokensOut)
	}

	fmt.Printf("    Total latency: %dms\n", summary.TotalLatency.Milliseconds())

	if len(summary.ToolsCalled) > 0 {
		fmt.Print("    Tools called: ")
		first := true
		for _, tool := range summary.ToolsCalled {
			if !first {
				fmt.Print(", ")
			}
			fmt.Print(tool)
			first = false
		}
		fmt.Println()
	}
}

// PrintComparison displays a comparison between current and baseline sessions.
func PrintComparison(comp *Comparison) {
	fmt.Println()
	fmt.Println("  Comparison with baseline:")

	// Compare call counts
	if comp.CallCountChanged {
		fmt.Printf("    ⚠ Call count changed: %d → %d\n",
			comp.BaselineCount,
			comp.CurrentCount)
	} else {
		fmt.Printf("    ✓ Call count unchanged: %d\n", comp.CurrentCount)
	}

	// New tools
	for _, tool := range comp.NewTools {
		fmt.Printf("    ⚠ New tool called: %s\n", tool)
	}

	// Removed tools
	for _, tool := range comp.RemovedTools {
		fmt.Printf("    ⚠ Tool no longer called: %s\n", tool)
	}

	// Model changes
	for _, change := range comp.ModelChanges {
		if change.IsNew {
			fmt.Printf("    ⚠ New model used: %s\n", change.Model)
		} else {
			fmt.Printf("    ⚠ Model %s usage changed: %d → %d\n",
				change.Model, change.BaselineCount, change.CurrentCount)
		}
	}

	// Token usage change
	if comp.TokenDiff != 0 {
		direction := "increased"
		diff := comp.TokenDiff
		if diff < 0 {
			direction = "decreased"
			diff = -diff
		}
		fmt.Printf("    ⚠ Token usage %s by %d\n", direction, diff)
	}
}
