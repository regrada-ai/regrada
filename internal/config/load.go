package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigYML  = "regrada.yml"
	defaultConfigYAML = "regrada.yaml"
)

func LoadProjectConfig(path string) (*ProjectConfig, error) {
	configPath, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ProjectConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := applyDefaults(&cfg, configPath); err != nil {
		return nil, err
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	if _, err := os.Stat(defaultConfigYML); err == nil {
		return defaultConfigYML, nil
	}
	if _, err := os.Stat(defaultConfigYAML); err == nil {
		return defaultConfigYAML, nil
	}
	return "", fmt.Errorf("config not found (looked for %s or %s)", defaultConfigYML, defaultConfigYAML)
}

func applyDefaults(cfg *ProjectConfig, configPath string) error {
	if cfg.Project.Root == "" {
		cfg.Project.Root = "."
	}
	if cfg.Project.Name == "" {
		dir := filepath.Dir(configPath)
		if dir == "." || dir == "" {
			if cwd, err := os.Getwd(); err == nil {
				dir = cwd
			}
		}
		cfg.Project.Name = filepath.Base(dir)
	}

	if len(cfg.Cases.Roots) == 0 {
		cfg.Cases.Roots = []string{"regrada/cases"}
	}
	if len(cfg.Cases.Include) == 0 {
		cfg.Cases.Include = []string{"**/*.yml", "**/*.yaml"}
	}
	if len(cfg.Cases.Exclude) == 0 {
		cfg.Cases.Exclude = []string{"**/README.*"}
	}
	if cfg.Cases.Defaults.Runs == 0 {
		cfg.Cases.Defaults.Runs = 3
	}
	if cfg.Cases.Defaults.TimeoutMS == 0 {
		cfg.Cases.Defaults.TimeoutMS = 30000
	}
	if cfg.Cases.Defaults.Concurrency == 0 {
		cfg.Cases.Defaults.Concurrency = 8
	}
	if cfg.Cases.Defaults.Sampling.Temperature == nil {
		v := 0.2
		cfg.Cases.Defaults.Sampling.Temperature = &v
	}
	if cfg.Cases.Defaults.Sampling.TopP == nil {
		v := 1.0
		cfg.Cases.Defaults.Sampling.TopP = &v
	}

	if cfg.Capture.Enabled == nil {
		v := true
		cfg.Capture.Enabled = &v
	}
	if cfg.Capture.Mode == "" {
		cfg.Capture.Mode = "proxy"
	}
	if cfg.Capture.Proxy.Listen == "" {
		cfg.Capture.Proxy.Listen = "127.0.0.1:4141"
	}
	if cfg.Capture.Proxy.Upstream.OpenAIBaseURL == "" {
		cfg.Capture.Proxy.Upstream.OpenAIBaseURL = "https://api.openai.com"
	}
	if cfg.Capture.Redact.Enabled == nil {
		v := true
		cfg.Capture.Redact.Enabled = &v
	}
	if len(cfg.Capture.Redact.Presets) == 0 {
		cfg.Capture.Redact.Presets = []string{"pii_basic", "secrets"}
	}

	if cfg.Providers.Default == "" {
		cfg.Providers.Default = "mock"
	}
	if cfg.Providers.OpenAI.APIKeyEnv == "" {
		cfg.Providers.OpenAI.APIKeyEnv = "OPENAI_API_KEY"
	}
	if cfg.Providers.OpenAI.BaseURLEnv == "" {
		cfg.Providers.OpenAI.BaseURLEnv = "OPENAI_BASE_URL"
	}
	if cfg.Providers.Anthropic.APIKeyEnv == "" {
		cfg.Providers.Anthropic.APIKeyEnv = "ANTHROPIC_API_KEY"
	}
	if cfg.Providers.Anthropic.BaseURLEnv == "" {
		cfg.Providers.Anthropic.BaseURLEnv = "ANTHROPIC_BASE_URL"
	}

	if cfg.Baseline.Mode == "" {
		cfg.Baseline.Mode = "git"
	}
	if cfg.Baseline.Git.Ref == "" {
		cfg.Baseline.Git.Ref = "origin/main"
	}
	if cfg.Baseline.Git.SnapshotDir == "" {
		cfg.Baseline.Git.SnapshotDir = ".regrada/snapshots"
	}
	if cfg.Baseline.Local.SnapshotDir == "" {
		cfg.Baseline.Local.SnapshotDir = ".regrada/snapshots"
	}

	if len(cfg.Report.Format) == 0 {
		cfg.Report.Format = []string{"summary", "markdown"}
	}
	if cfg.Report.Markdown.Path == "" {
		cfg.Report.Markdown.Path = ".regrada/report.md"
	}
	if cfg.Report.JUnit.Path == "" {
		cfg.Report.JUnit.Path = ".regrada/junit.xml"
	}
	if cfg.Report.StoreArtifacts == nil {
		v := true
		cfg.Report.StoreArtifacts = &v
	}

	if len(cfg.CI.FailOn) == 0 {
		cfg.CI.FailOn = []FailOnSeverity{{Severity: "error"}}
	}
	if cfg.CI.CommentOnPR == nil {
		v := true
		cfg.CI.CommentOnPR = &v
	}
	for i, entry := range cfg.CI.FailOn {
		if entry.Severity != "error" && entry.Severity != "warn" {
			return fmt.Errorf("ci.fail_on[%d].severity must be error or warn", i)
		}
	}

	if cfg.Record.SessionDir == "" {
		cfg.Record.SessionDir = ".regrada/sessions"
	}
	if cfg.Record.TracesDir == "" {
		cfg.Record.TracesDir = ".regrada/traces"
	}
	if cfg.Record.Accept.OutputDir == "" {
		cfg.Record.Accept.OutputDir = "regrada/cases/recorded"
	}
	if len(cfg.Record.Accept.DefaultTags) == 0 {
		cfg.Record.Accept.DefaultTags = []string{"recorded"}
	}
	if cfg.Record.Accept.InferAsserts == nil {
		v := true
		cfg.Record.Accept.InferAsserts = &v
	}
	if cfg.Record.Accept.Normalize.TrimWhitespace == nil {
		v := true
		cfg.Record.Accept.Normalize.TrimWhitespace = &v
	}
	if cfg.Record.Accept.Normalize.DropVolatileFields == nil {
		v := true
		cfg.Record.Accept.Normalize.DropVolatileFields = &v
	}

	return nil
}

func validateConfig(cfg *ProjectConfig) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported version %d", cfg.Version)
	}
	if cfg.Capture.Mode != "proxy" && cfg.Capture.Mode != "off" {
		return fmt.Errorf("capture.mode must be proxy or off")
	}
	if cfg.Baseline.Mode != "git" && cfg.Baseline.Mode != "local" {
		return fmt.Errorf("baseline.mode must be git or local")
	}
	if cfg.Providers.Default != "" && !isValidProvider(cfg.Providers.Default) {
		return fmt.Errorf("providers.default must be one of openai, anthropic, azure_openai, bedrock, mock")
	}

	for i, policy := range cfg.Policies {
		if policy.ID == "" {
			return fmt.Errorf("policies[%d].id is required", i)
		}
		if policy.Severity == "" {
			cfg.Policies[i].Severity = "error"
		} else if policy.Severity != "error" && policy.Severity != "warn" {
			return fmt.Errorf("policies[%d].severity must be error or warn", i)
		}
		if policy.Check.Type == "json_valid" && policy.Check.Extractor == "" {
			cfg.Policies[i].Check.Extractor = "assistant_text"
		}
		if err := validatePolicyCheck(policy.Check); err != nil {
			return fmt.Errorf("policies[%d].check: %w", i, err)
		}
	}

	return nil
}

func isValidProvider(value string) bool {
	switch value {
	case "openai", "anthropic", "azure_openai", "bedrock", "mock":
		return true
	default:
		return false
	}
}

func validatePolicyCheck(check PolicyCheck) error {
	if check.Type == "" {
		return errors.New("type is required")
	}
	switch check.Type {
	case "json_valid":
		if check.Extractor == "" {
			check.Extractor = "assistant_text"
		}
	case "json_schema":
		if check.Schema == "" {
			return errors.New("schema is required")
		}
	case "text_contains", "text_not_contains":
		if len(check.Phrases) == 0 {
			return errors.New("phrases is required")
		}
	case "pii_leak":
		if check.Detector == "" {
			return errors.New("detector is required")
		}
	case "variance":
		if check.Metric == "" {
			return errors.New("metric is required")
		}
	case "refusal_rate":
		if check.Max == nil && check.MaxDelta == nil {
			return errors.New("max or max_delta is required")
		}
	case "latency":
		if check.LatencyP95 == nil {
			return errors.New("p95_ms is required")
		}
	default:
		return fmt.Errorf("unsupported check type %q", check.Type)
	}
	return nil
}

func DefaultConfig(projectName string) ProjectConfig {
	cfg := ProjectConfig{
		Version: 1,
		Project: ProjectMeta{Name: projectName, Root: "."},
	}
	_ = applyDefaults(&cfg, filepath.Join(projectName, defaultConfigYML))
	return cfg
}
