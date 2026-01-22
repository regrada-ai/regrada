package cmd

import (
	"fmt"
	"time"

	"github.com/matias/regrada/internal/baseline"
	"github.com/matias/regrada/internal/cases"
	"github.com/matias/regrada/internal/config"
	"github.com/matias/regrada/internal/diff"
	"github.com/matias/regrada/internal/eval"
	"github.com/matias/regrada/internal/git"
	"github.com/matias/regrada/internal/model"
	"github.com/matias/regrada/internal/policy"
	"github.com/matias/regrada/internal/providers"
	"github.com/matias/regrada/internal/report"
	"github.com/spf13/cobra"
)

var testConfigPath string

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run evals, diff against baselines, apply policies",
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringVarP(&testConfigPath, "config", "c", "", "Path to config file (default: regrada.yml/regrada.yaml)")
}

func runTest(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(testConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	caseList, err := cases.DiscoverCases(cfg)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}
	if len(caseList) == 0 {
		return ExitError{Code: 3, Err: fmt.Errorf("no cases discovered")}
	}

	provider, err := providers.Resolve(cfg)
	if err != nil {
		return ExitError{Code: 1, Err: err}
	}

	var baselineStore baseline.Store
	switch cfg.Baseline.Mode {
	case "git":
		baselineStore = baseline.NewGitStore(cfg.Baseline.Git.Ref, cfg.Baseline.Git.SnapshotDir, git.NewExecClient())
	case "local":
		baselineStore = baseline.NewLocalStore(cfg.Baseline.Local.SnapshotDir)
	default:
		return ExitError{Code: 3, Err: fmt.Errorf("unsupported baseline mode %q", cfg.Baseline.Mode)}
	}

	evalSettings := eval.EvalSettings{
		Runs:    cfg.Cases.Defaults.Runs,
		Timeout: time.Duration(cfg.Cases.Defaults.TimeoutMS) * time.Millisecond,
		Sampling: &model.SamplingParams{
			Temperature:     cfg.Cases.Defaults.Sampling.Temperature,
			TopP:            cfg.Cases.Defaults.Sampling.TopP,
			MaxOutputTokens: cfg.Cases.Defaults.Sampling.MaxOutputTokens,
		},
	}

	modelName := resolveModel(cfg, provider.Name())
	var summaries []report.CaseSummary
	for _, c := range caseList {
		result, err := eval.RunCase(cmd.Context(), c, evalSettings, provider)
		if err != nil {
			return ExitError{Code: 5, Err: err}
		}
		result.Model = modelName

		systemPrompt := extractSystemPrompt(c)
		sampling := resolveSampling(c, evalSettings)
		baselineKey, paramsHash, err := baseline.Key(provider.Name(), modelName, sampling, systemPrompt)
		if err != nil {
			return ExitError{Code: 1, Err: err}
		}
		base, err := baselineStore.Load(c.ID, baselineKey)
		if err != nil {
			return ExitError{Code: 4, Err: err}
		}
		base.ParamsHash = paramsHash

		delta := diff.Diff(result, base)
		violations := policy.Evaluate(cfg.Policies, c, result, delta)
		summaries = append(summaries, report.CaseSummary{
			CaseID:     c.ID,
			Result:     result,
			Diff:       delta,
			Violations: violations,
		})
	}

	summary := report.BuildSummary(summaries)
	report.SortCases(&summary)

	for _, format := range cfg.Report.Format {
		switch format {
		case "summary":
			fmt.Printf("Total: %d | Passed: %d | Warned: %d | Failed: %d\n", summary.Total, summary.Passed, summary.Warned, summary.Failed)
		case "markdown":
			if err := report.WriteMarkdown(summary, cfg.Report.Markdown.Path); err != nil {
				return ExitError{Code: 1, Err: err}
			}
		case "junit":
			if err := report.WriteJUnit(summary, cfg.Report.JUnit.Path); err != nil {
				return ExitError{Code: 1, Err: err}
			}
		}
	}

	if shouldFail(cfg, summary) {
		return ExitError{Code: 2, Err: fmt.Errorf("policy violations")}
	}
	return nil
}

func shouldFail(cfg *config.ProjectConfig, summary report.RunSummary) bool {
	severities := make(map[string]bool)
	for _, entry := range cfg.CI.FailOn {
		severities[entry.Severity] = true
	}
	for _, c := range summary.Cases {
		for _, v := range c.Violations {
			if severities[v.Severity] {
				return true
			}
		}
	}
	return false
}
