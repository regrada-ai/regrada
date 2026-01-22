package report

import (
	"sort"

	"github.com/matias/regrada/internal/diff"
	"github.com/matias/regrada/internal/eval"
	"github.com/matias/regrada/internal/policy"
)

type CaseSummary struct {
	CaseID     string
	Result     eval.CaseResult
	Diff       diff.DiffResult
	Violations []policy.Violation
}

type RunSummary struct {
	Total  int
	Passed int
	Warned int
	Failed int
	Cases  []CaseSummary
}

func BuildSummary(cases []CaseSummary) RunSummary {
	summary := RunSummary{Total: len(cases)}
	for _, c := range cases {
		severity := maxSeverity(c.Violations)
		switch severity {
		case "error":
			summary.Failed++
		case "warn":
			summary.Warned++
		default:
			summary.Passed++
		}
		summary.Cases = append(summary.Cases, c)
	}
	return summary
}

func maxSeverity(violations []policy.Violation) string {
	severity := ""
	for _, v := range violations {
		switch v.Severity {
		case "error":
			return "error"
		case "warn":
			severity = "warn"
		}
	}
	return severity
}

func SortCases(summary *RunSummary) {
	sort.Slice(summary.Cases, func(i, j int) bool {
		return summary.Cases[i].CaseID < summary.Cases[j].CaseID
	})
}
