package diff

import (
	"strings"

	"github.com/regrada-ai/regrada/internal/baseline"
	"github.com/regrada-ai/regrada/internal/eval"
)

type DiffResult struct {
	CaseID      string             `json:"case_id"`
	MetricDelta map[string]float64 `json:"metric_delta"`
	TextDelta   TextDelta          `json:"text_delta"`
	JSONDelta   JSONDelta          `json:"json_delta"`
}

type TextDelta struct {
	TokenJaccard float64 `json:"token_jaccard"`
}

type JSONDelta struct {
	ChangedPaths []string `json:"changed_paths,omitempty"`
}

func Diff(current eval.CaseResult, base baseline.Baseline) DiffResult {
	delta := map[string]float64{
		"pass_rate":       current.Aggregates.PassRate - base.Aggregates.PassRate,
		"latency_p95_ms":  float64(current.Aggregates.LatencyP95MS - base.Aggregates.LatencyP95MS),
		"refusal_rate":    current.Aggregates.RefusalRate - base.Aggregates.RefusalRate,
		"json_valid_rate": current.Aggregates.JSONValidRate - base.Aggregates.JSONValidRate,
	}

	textDelta := TextDelta{}
	if len(current.Runs) > 0 {
		textDelta.TokenJaccard = tokenJaccard(base.GoldenText, current.Runs[0].OutputText)
	}

	return DiffResult{
		CaseID:      current.CaseID,
		MetricDelta: delta,
		TextDelta:   textDelta,
	}
}

func tokenJaccard(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1
	}
	intersection := 0
	union := len(setA)
	for token := range setB {
		if setA[token] {
			intersection++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range strings.Fields(strings.ToLower(text)) {
		set[token] = true
	}
	return set
}
