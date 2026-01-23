package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/diff"
	"github.com/regrada-ai/regrada/internal/eval"
)

type Violation struct {
	PolicyID string `json:"policy_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

func Evaluate(policies []config.Policy, c cases.Case, res eval.CaseResult, d diff.DiffResult) []Violation {
	var violations []Violation
	for _, policy := range policies {
		if !matchesScope(policy.Scope, c, res) {
			continue
		}
		if v := evaluatePolicy(policy, res, d); v != nil {
			violations = append(violations, *v)
		}
	}
	return violations
}

func matchesScope(scope *config.PolicyScope, c cases.Case, res eval.CaseResult) bool {
	if scope == nil {
		return true
	}
	if len(scope.Tags) > 0 {
		if !containsAny(c.Tags, scope.Tags) {
			return false
		}
	}
	if len(scope.IDs) > 0 {
		if !containsAny([]string{c.ID}, scope.IDs) {
			return false
		}
	}
	if len(scope.Providers) > 0 {
		if !containsAny([]string{res.Provider}, scope.Providers) {
			return false
		}
	}
	return true
}

func evaluatePolicy(policy config.Policy, res eval.CaseResult, d diff.DiffResult) *Violation {
	check := policy.Check
	switch check.Type {
	case "json_valid":
		threshold := 1.0
		if check.MinPassRate != nil {
			threshold = *check.MinPassRate
		}
		if res.Aggregates.JSONValidRate < threshold {
			return violation(policy, fmt.Sprintf("json valid rate %.2f below %.2f", res.Aggregates.JSONValidRate, threshold))
		}
	case "text_contains":
		threshold := 1.0
		if check.MinPassRate != nil {
			threshold = *check.MinPassRate
		}
		passRate := textContainsRate(res.Runs, check.Phrases)
		if passRate < threshold {
			return violation(policy, fmt.Sprintf("text contains rate %.2f below %.2f", passRate, threshold))
		}
	case "text_not_contains":
		maxIncidents := 0
		if check.MaxIncidents != nil {
			maxIncidents = *check.MaxIncidents
		}
		incidents := textNotContainsIncidents(res.Runs, check.Phrases)
		if incidents > maxIncidents {
			return violation(policy, fmt.Sprintf("text not_contains incidents %d above %d", incidents, maxIncidents))
		}
	case "pii_leak":
		maxIncidents := 0
		if check.MaxIncidents != nil {
			maxIncidents = *check.MaxIncidents
		}
		incidents := piiIncidents(res.Runs, check.Detector)
		if incidents > maxIncidents {
			return violation(policy, fmt.Sprintf("pii incidents %d above %d", incidents, maxIncidents))
		}
	case "variance":
		if check.MaxP95 == nil {
			return violation(policy, "max_p95 is required")
		}
		variance := 1 - d.TextDelta.TokenJaccard
		if variance > *check.MaxP95 {
			return violation(policy, fmt.Sprintf("variance %.2f above %.2f", variance, *check.MaxP95))
		}
	case "refusal_rate":
		if check.Max != nil && res.Aggregates.RefusalRate > *check.Max {
			return violation(policy, fmt.Sprintf("refusal rate %.2f above %.2f", res.Aggregates.RefusalRate, *check.Max))
		}
		if check.MaxDelta != nil {
			delta := d.MetricDelta["refusal_rate"]
			if delta > *check.MaxDelta {
				return violation(policy, fmt.Sprintf("refusal delta %.2f above %.2f", delta, *check.MaxDelta))
			}
		}
	case "latency":
		if check.LatencyP95 == nil {
			return violation(policy, "p95_ms is required")
		}
		if check.LatencyP95.Max != nil && res.Aggregates.LatencyP95MS > *check.LatencyP95.Max {
			return violation(policy, fmt.Sprintf("latency p95 %d above %d", res.Aggregates.LatencyP95MS, *check.LatencyP95.Max))
		}
		if check.LatencyP95.MaxDelta != nil {
			delta := d.MetricDelta["latency_p95_ms"]
			if delta > float64(*check.LatencyP95.MaxDelta) {
				return violation(policy, fmt.Sprintf("latency delta %.0f above %d", delta, *check.LatencyP95.MaxDelta))
			}
		}
	case "json_schema":
		return violation(policy, "json schema checks not implemented")
	default:
		return violation(policy, fmt.Sprintf("unsupported check %q", check.Type))
	}
	return nil
}

func violation(policy config.Policy, msg string) *Violation {
	severity := policy.Severity
	if severity == "" {
		severity = "error"
	}
	return &Violation{
		PolicyID: policy.ID,
		Severity: severity,
		Message:  msg,
	}
}

func containsAny(haystack, needles []string) bool {
	set := make(map[string]bool)
	for _, v := range haystack {
		set[v] = true
	}
	for _, needle := range needles {
		if set[needle] {
			return true
		}
	}
	return false
}

func textContainsRate(runs []eval.RunResult, phrases []string) float64 {
	if len(runs) == 0 {
		return 0
	}
	pass := 0
	for _, run := range runs {
		matched := true
		for _, phrase := range phrases {
			if !strings.Contains(run.OutputText, phrase) {
				matched = false
				break
			}
		}
		if matched {
			pass++
		}
	}
	return float64(pass) / float64(len(runs))
}

func textNotContainsIncidents(runs []eval.RunResult, phrases []string) int {
	count := 0
	for _, run := range runs {
		for _, phrase := range phrases {
			if strings.Contains(run.OutputText, phrase) {
				count++
				break
			}
		}
	}
	return count
}

func piiIncidents(runs []eval.RunResult, detector string) int {
	patterns := piiPatterns(detector)
	count := 0
	for _, run := range runs {
		for _, re := range patterns {
			if re.MatchString(run.OutputText) {
				count++
				break
			}
		}
	}
	return count
}

func piiPatterns(detector string) []*regexp.Regexp {
	switch detector {
	case "pii_strict":
		return []*regexp.Regexp{
			regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
			regexp.MustCompile(`\+?\d[\d\s().-]{7,}\d`),
			regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		}
	default:
		return []*regexp.Regexp{
			regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
			regexp.MustCompile(`\+?\d[\d\s().-]{7,}\d`),
		}
	}
}
