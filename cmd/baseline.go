package cmd

import (
	"fmt"
	"time"

	"github.com/regrada-ai/regrada/internal/baseline"
	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/eval"
	"github.com/regrada-ai/regrada/internal/model"
	"github.com/regrada-ai/regrada/internal/providers"
	"github.com/spf13/cobra"
)

var baselineConfigPath string

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Generate baseline snapshots for discovered cases",
	RunE:  runBaseline,
}

func init() {
	rootCmd.AddCommand(baselineCmd)

	baselineCmd.Flags().StringVarP(&baselineConfigPath, "config", "c", "", "Path to config file (default: regrada.yml/regrada.yaml)")
}

func runBaseline(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(baselineConfigPath)
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

	baselineStore := baseline.NewLocalStore(cfg.Baseline.Local.SnapshotDir)

	evalSettings := eval.EvalSettings{
		Runs:    1,
		Timeout: time.Duration(cfg.Cases.Defaults.TimeoutMS) * time.Millisecond,
		Sampling: &model.SamplingParams{
			Temperature:     cfg.Cases.Defaults.Sampling.Temperature,
			TopP:            cfg.Cases.Defaults.Sampling.TopP,
			MaxOutputTokens: cfg.Cases.Defaults.Sampling.MaxOutputTokens,
		},
	}

	modelName := resolveModel(cfg, provider.Name())
	written := 0
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
		base := baseline.FromResult(result, baselineKey, paramsHash)
		if err := baselineStore.Save(c.ID, baselineKey, base); err != nil {
			return ExitError{Code: 1, Err: err}
		}
		written++
	}

	fmt.Printf("Wrote baselines for %d cases to %s\n", written, cfg.Baseline.Local.SnapshotDir)
	return nil
}
