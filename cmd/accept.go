package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/matias/regrada/internal/accept"
	"github.com/matias/regrada/internal/baseline"
	"github.com/matias/regrada/internal/cases"
	"github.com/matias/regrada/internal/config"
	"github.com/matias/regrada/internal/eval"
	"github.com/matias/regrada/internal/model"
	"github.com/matias/regrada/internal/providers"
	"github.com/matias/regrada/internal/record"
	"github.com/matias/regrada/internal/trace"
	"github.com/matias/regrada/internal/util"
	"github.com/spf13/cobra"
)

var (
	acceptConfigPath string
	acceptSession    string
)

var acceptCmd = &cobra.Command{
	Use:   "accept",
	Short: "Convert recorded traces into cases and baselines",
	RunE:  runAccept,
}

func init() {
	rootCmd.AddCommand(acceptCmd)

	acceptCmd.Flags().StringVarP(&acceptConfigPath, "config", "c", "", "Path to config file (default: regrada.yml/regrada.yaml)")
	acceptCmd.Flags().StringVar(&acceptSession, "session", "", "Session file to accept (default: latest)")
}

func runAccept(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(acceptConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	sessionPath := acceptSession
	if sessionPath == "" {
		sessionPath, err = record.LatestSession(cfg.Record.SessionDir)
		if err != nil {
			return ExitError{Code: 1, Err: err}
		}
	}

	session, err := record.LoadSession(sessionPath)
	if err != nil {
		return ExitError{Code: 1, Err: err}
	}
	if len(session.TraceIDs) == 0 {
		return ExitError{Code: 1, Err: fmt.Errorf("session has no traces")}
	}

	store := trace.NewLocalStore(cfg.Record.TracesDir)
	provider, err := providers.Resolve(cfg)
	if err != nil {
		return ExitError{Code: 1, Err: err}
	}

	evalSettings := eval.EvalSettings{
		Runs:    1,
		Timeout: time.Duration(cfg.Cases.Defaults.TimeoutMS) * time.Millisecond,
		Sampling: &model.SamplingParams{
			Temperature:     cfg.Cases.Defaults.Sampling.Temperature,
			TopP:            cfg.Cases.Defaults.Sampling.TopP,
			MaxOutputTokens: cfg.Cases.Defaults.Sampling.MaxOutputTokens,
		},
	}

	converter := accept.ConvertOptions{
		DefaultTags:  cfg.Record.Accept.DefaultTags,
		InferAsserts: cfg.Record.Accept.InferAsserts == nil || *cfg.Record.Accept.InferAsserts,
		Normalize: accept.NormalizeOptions{
			TrimWhitespace:     cfg.Record.Accept.Normalize.TrimWhitespace == nil || *cfg.Record.Accept.Normalize.TrimWhitespace,
			DropVolatileFields: cfg.Record.Accept.Normalize.DropVolatileFields == nil || *cfg.Record.Accept.Normalize.DropVolatileFields,
		},
	}

	baselineStore := baseline.NewLocalStore(cfg.Baseline.Local.SnapshotDir)
	modelName := resolveModel(cfg, provider.Name())

	for _, traceID := range session.TraceIDs {
		tr, err := store.Read(traceID)
		if err != nil {
			return ExitError{Code: 1, Err: err}
		}
		c, err := accept.FromTrace(tr, converter)
		if err != nil {
			return ExitError{Code: 1, Err: err}
		}
		if err := cases.ValidateCase(c); err != nil {
			return ExitError{Code: 1, Err: err}
		}
		casePath := filepath.Join(cfg.Record.Accept.OutputDir, util.Slugify(c.ID)+".yml")
		if err := cases.WriteCase(casePath, c); err != nil {
			return ExitError{Code: 1, Err: err}
		}

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

		fmt.Printf("Accepted case %s\n", c.ID)
	}

	fmt.Printf("Accepted %d traces from %s\n", len(session.TraceIDs), sessionPath)
	return nil
}

func resolveModel(cfg *config.ProjectConfig, provider string) string {
	switch provider {
	case "openai":
		return cfg.Providers.OpenAI.Model
	case "anthropic":
		return cfg.Providers.Anthropic.Model
	case "azure_openai":
		return cfg.Providers.AzureOpenAI.Deployment
	case "bedrock":
		return cfg.Providers.Bedrock.ModelID
	default:
		return ""
	}
}

func extractSystemPrompt(c cases.Case) string {
	var parts []string
	for _, msg := range c.Request.Messages {
		if msg.Role == "system" {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func resolveSampling(c cases.Case, settings eval.EvalSettings) *model.SamplingParams {
	if c.Request.Params != nil {
		return &model.SamplingParams{
			Temperature:     c.Request.Params.Temperature,
			TopP:            c.Request.Params.TopP,
			MaxOutputTokens: c.Request.Params.MaxOutputTokens,
			Stop:            c.Request.Params.Stop,
		}
	}
	if c.Sampling != nil {
		return &model.SamplingParams{
			Temperature:     c.Sampling.Temperature,
			TopP:            c.Sampling.TopP,
			MaxOutputTokens: c.Sampling.MaxOutputTokens,
		}
	}
	return settings.Sampling
}
