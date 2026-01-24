package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/regrada-ai/regrada/internal/backend"
	"github.com/regrada-ai/regrada/internal/baseline"
	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/diff"
	"github.com/regrada-ai/regrada/internal/eval"
	"github.com/regrada-ai/regrada/internal/git"
	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/policy"
	"github.com/regrada-ai/regrada/internal/providers"
	"github.com/regrada-ai/regrada/internal/report"
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
	for i, c := range caseList {
		fmt.Printf("[%d/%d] Running test: %s\n", i+1, len(caseList), c.ID)

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

		summary := report.CaseSummary{
			CaseID:     c.ID,
			Result:     result,
			Diff:       delta,
			Violations: violations,
		}
		summaries = append(summaries, summary)

		// Display test result
		status := "✓ PASS"
		hasFailed := false
		hasWarned := false
		for _, v := range violations {
			if v.Severity == "error" {
				hasFailed = true
			} else if v.Severity == "warn" {
				hasWarned = true
			}
		}
		if hasFailed {
			status = "✗ FAIL"
		} else if hasWarned {
			status = "⚠ WARN"
		}
		fmt.Printf("  %s (pass_rate: %.0f%%, p95: %dms)\n", status, result.Aggregates.PassRate*100, result.Aggregates.LatencyP95MS)

		// Show assertion failures
		for _, run := range result.Runs {
			if !run.Pass && run.Error != "" {
				fmt.Printf("    run %d failed: %s\n", run.RunID, run.Error)
			}
		}

		// Show policy violations
		if len(violations) > 0 {
			for _, v := range violations {
				fmt.Printf("    - %s: %s\n", v.Severity, v.Message)
			}
		}
	}

	summary := report.BuildSummary(summaries)
	report.SortCases(&summary)

	fmt.Println() // Add blank line before final summary
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

	// Upload to backend if enabled
	if cfg.Backend.Enabled != nil && *cfg.Backend.Enabled {
		if cfg.Backend.Upload.TestResults != nil && *cfg.Backend.Upload.TestResults {
			apiKey := os.Getenv(cfg.Backend.APIKeyEnv)
			if apiKey == "" {
				fmt.Printf("Warning: Backend upload enabled but %s environment variable not set\n", cfg.Backend.APIKeyEnv)
			} else if cfg.Backend.ProjectID == "" {
				fmt.Printf("Warning: Backend upload enabled but project_id not configured\n")
			} else {
				if err := uploadTestResults(cmd, cfg, summary, summaries); err != nil {
					fmt.Printf("Warning: failed to upload results to backend: %v\n", err)
				} else {
					fmt.Printf("✓ Results uploaded to Regrada backend\n")
				}
			}
		} else {
			fmt.Printf("Debug: Backend enabled but test_results upload is disabled\n")
		}
	} else {
		fmt.Printf("Debug: Backend upload is disabled in config\n")
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

func uploadTestResults(cmd *cobra.Command, cfg *config.ProjectConfig, summary report.RunSummary, summaries []report.CaseSummary) error {
	client := backend.NewClient(os.Getenv(cfg.Backend.APIKeyEnv), cfg.Backend.ProjectID)
	ctx := cmd.Context()

	// Collect git context
	gitClient := git.NewExecClient()
	gitSHA, _ := gitClient.GetCurrentCommit()
	gitBranch, _ := gitClient.GetCurrentBranch()
	gitMessage := ""
	if gitSHA != "" {
		gitMessage, _ = gitClient.GetCommitMessage(gitSHA)
	}

	// Detect CI environment
	ciProvider := ""
	ciPRNumber := 0
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		ciProvider = "github_actions"
		// Could parse PR number from GITHUB_REF if needed
	} else if os.Getenv("CIRCLECI") == "true" {
		ciProvider = "circleci"
	} else if os.Getenv("JENKINS_HOME") != "" {
		ciProvider = "jenkins"
	}

	// Generate unique run ID
	runID := generateRunID()

	// Convert summaries to map format for JSON
	results := make([]map[string]interface{}, len(summaries))
	for i, s := range summaries {
		results[i] = map[string]interface{}{
			"case_id":         s.CaseID,
			"pass_rate":       s.Result.Aggregates.PassRate,
			"latency_p95":     s.Result.Aggregates.LatencyP95MS,
			"refusal_rate":    s.Result.Aggregates.RefusalRate,
			"json_valid_rate": s.Result.Aggregates.JSONValidRate,
			"violations":      s.Violations,
		}
	}

	testRunData := map[string]interface{}{
		"run_id":             runID,
		"timestamp":          time.Now(),
		"git_sha":            gitSHA,
		"git_branch":         gitBranch,
		"git_commit_message": gitMessage,
		"ci_provider":        ciProvider,
		"ci_pr_number":       ciPRNumber,
		"total_cases":        summary.Total,
		"passed_cases":       summary.Passed,
		"failed_cases":       summary.Failed,
		"results":            results,
		"status":             "completed",
	}

	return client.UploadTestRun(ctx, testRunData)
}

func generateRunID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "run_" + hex.EncodeToString(b)
}
