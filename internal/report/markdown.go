package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WriteMarkdown(summary RunSummary, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# Regrada Report\n\n")
	fmt.Fprintf(&b, "Total: %d | Passed: %d | Warned: %d | Failed: %d\n\n", summary.Total, summary.Passed, summary.Warned, summary.Failed)

	b.WriteString("## Top Diffs\n")
	for _, entry := range topDiffs(summary, 5) {
		fmt.Fprintf(&b, "- %s: token_jaccard=%.2f\n", entry.CaseID, entry.Diff.TextDelta.TokenJaccard)
	}
	b.WriteString("\n")

	b.WriteString("## Violations\n")
	violations := collectViolations(summary)
	if len(violations) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, v := range violations {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", v.Severity, v.PolicyID, v.Message)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

func topDiffs(summary RunSummary, limit int) []CaseSummary {
	cases := append([]CaseSummary{}, summary.Cases...)
	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Diff.TextDelta.TokenJaccard < cases[j].Diff.TextDelta.TokenJaccard
	})
	if len(cases) > limit {
		cases = cases[:limit]
	}
	return cases
}

func collectViolations(summary RunSummary) []ViolationInfo {
	var out []ViolationInfo
	for _, c := range summary.Cases {
		for _, v := range c.Violations {
			out = append(out, ViolationInfo{PolicyID: v.PolicyID, Severity: v.Severity, Message: v.Message})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity == out[j].Severity {
			return out[i].PolicyID < out[j].PolicyID
		}
		return severityRank(out[i].Severity) < severityRank(out[j].Severity)
	})
	return out
}

type ViolationInfo struct {
	PolicyID string
	Severity string
	Message  string
}

func severityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warn":
		return 1
	default:
		return 2
	}
}
