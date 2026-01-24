package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// applyDefaults sets default values for unspecified configuration fields
func applyDefaults(cfg *ProjectConfig, configPath string) error {
	applyProjectDefaults(cfg, configPath)
	applyCasesDefaults(cfg)
	applyCaptureDefaults(cfg)
	applyProvidersDefaults(cfg)
	applyBaselineDefaults(cfg)
	applyReportDefaults(cfg)
	if err := applyCIDefaults(cfg); err != nil {
		return err
	}
	applyRecordDefaults(cfg)
	applyBackendDefaults(cfg)
	return nil
}

func applyProjectDefaults(cfg *ProjectConfig, configPath string) {
	if cfg.Project.Root == "" {
		cfg.Project.Root = "."
	}
	if cfg.Project.Name == "" {
		cfg.Project.Name = deriveProjectName(configPath)
	}
}

func deriveProjectName(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "." || dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}
	return filepath.Base(dir)
}

func applyCasesDefaults(cfg *ProjectConfig) {
	setDefaultSlice(&cfg.Cases.Roots, []string{"regrada/cases"})
	setDefaultSlice(&cfg.Cases.Include, []string{"**/*.yml", "**/*.yaml"})
	setDefaultSlice(&cfg.Cases.Exclude, []string{"**/README.*"})

	setDefaultInt(&cfg.Cases.Defaults.Runs, 3)
	setDefaultInt(&cfg.Cases.Defaults.TimeoutMS, 30000)
	setDefaultInt(&cfg.Cases.Defaults.Concurrency, 8)

	setDefaultFloat64Ptr(&cfg.Cases.Defaults.Sampling.Temperature, 0.2)
	setDefaultFloat64Ptr(&cfg.Cases.Defaults.Sampling.TopP, 1.0)
}

func applyCaptureDefaults(cfg *ProjectConfig) {
	setDefaultBoolPtr(&cfg.Capture.Enabled, true)
	setDefaultString(&cfg.Capture.Mode, "proxy")
	setDefaultString(&cfg.Capture.Proxy.Listen, "127.0.0.1:8080")
	setDefaultString(&cfg.Capture.Proxy.Mode, "forward")
	setDefaultString(&cfg.Capture.Proxy.CAPath, ".regrada/ca")

	// Auto-derive allow hosts from provider base URLs
	if len(cfg.Capture.Proxy.AllowHosts) == 0 && cfg.Capture.Proxy.Mode == "forward" {
		cfg.Capture.Proxy.AllowHosts = deriveAllowedHosts(cfg)
	}

	// For reverse proxy mode
	if cfg.Capture.Proxy.Mode == "reverse" {
		setDefaultString(&cfg.Capture.Proxy.Upstream.OpenAIBaseURL, "https://api.openai.com")
	}

	setDefaultBoolPtr(&cfg.Capture.Redact.Enabled, true)
	setDefaultSlice(&cfg.Capture.Redact.Presets, []string{"pii_basic", "secrets"})
}

func applyProvidersDefaults(cfg *ProjectConfig) {
	setDefaultString(&cfg.Providers.Default, "mock")

	setDefaultString(&cfg.Providers.OpenAI.APIKeyEnv, "OPENAI_API_KEY")
	setDefaultString(&cfg.Providers.OpenAI.BaseURLEnv, "OPENAI_BASE_URL")

	setDefaultString(&cfg.Providers.Anthropic.APIKeyEnv, "ANTHROPIC_API_KEY")
	setDefaultString(&cfg.Providers.Anthropic.BaseURLEnv, "ANTHROPIC_BASE_URL")
}

func applyBaselineDefaults(cfg *ProjectConfig) {
	setDefaultString(&cfg.Baseline.Mode, "git")
	setDefaultString(&cfg.Baseline.Git.Ref, "origin/main")
	setDefaultString(&cfg.Baseline.Git.SnapshotDir, ".regrada/snapshots")
	setDefaultString(&cfg.Baseline.Local.SnapshotDir, ".regrada/snapshots")
}

func applyReportDefaults(cfg *ProjectConfig) {
	setDefaultSlice(&cfg.Report.Format, []string{"summary", "markdown"})
	setDefaultString(&cfg.Report.Markdown.Path, ".regrada/report.md")
	setDefaultString(&cfg.Report.JUnit.Path, ".regrada/junit.xml")
	setDefaultBoolPtr(&cfg.Report.StoreArtifacts, true)
}

func applyCIDefaults(cfg *ProjectConfig) error {
	setDefaultSlice(&cfg.CI.FailOn, []FailOnSeverity{{Severity: "error"}})
	setDefaultBoolPtr(&cfg.CI.CommentOnPR, true)

	// Validate fail_on severities while applying defaults
	for i, entry := range cfg.CI.FailOn {
		if entry.Severity != "error" && entry.Severity != "warn" {
			return fmt.Errorf("ci.fail_on[%d].severity must be error or warn", i)
		}
	}
	return nil
}

func applyRecordDefaults(cfg *ProjectConfig) {
	setDefaultString(&cfg.Record.SessionDir, ".regrada/sessions")
	setDefaultString(&cfg.Record.TracesDir, ".regrada/traces")
	setDefaultString(&cfg.Record.Accept.OutputDir, "regrada/cases/recorded")
	setDefaultSlice(&cfg.Record.Accept.DefaultTags, []string{"recorded"})

	setDefaultBoolPtr(&cfg.Record.Accept.InferAsserts, true)
	setDefaultBoolPtr(&cfg.Record.Accept.Normalize.TrimWhitespace, true)
	setDefaultBoolPtr(&cfg.Record.Accept.Normalize.DropVolatileFields, true)
}

func applyBackendDefaults(cfg *ProjectConfig) {
	setDefaultBoolPtr(&cfg.Backend.Enabled, false) // Disabled by default
	setDefaultString(&cfg.Backend.APIKeyEnv, "REGRADA_API_KEY")

	setDefaultBoolPtr(&cfg.Backend.Upload.Traces, true)
	setDefaultBoolPtr(&cfg.Backend.Upload.TestResults, true)
	setDefaultBoolPtr(&cfg.Backend.Upload.Async, false)
	setDefaultInt(&cfg.Backend.Upload.BatchSize, 50)
}

// Helper functions to reduce repetition

func setDefaultString(field *string, defaultValue string) {
	if *field == "" {
		*field = defaultValue
	}
}

func setDefaultInt(field *int, defaultValue int) {
	if *field == 0 {
		*field = defaultValue
	}
}

func setDefaultBoolPtr(field **bool, defaultValue bool) {
	if *field == nil {
		v := defaultValue
		*field = &v
	}
}

func setDefaultFloat64Ptr(field **float64, defaultValue float64) {
	if *field == nil {
		v := defaultValue
		*field = &v
	}
}

func setDefaultSlice[T any](field *[]T, defaultValue []T) {
	if len(*field) == 0 {
		*field = defaultValue
	}
}

// deriveAllowedHosts extracts hostnames from provider base URLs
func deriveAllowedHosts(cfg *ProjectConfig) []string {
	hosts := make(map[string]bool)

	// Extract from OpenAI
	if cfg.Providers.OpenAI.BaseURL != "" {
		if host := extractHost(cfg.Providers.OpenAI.BaseURL); host != "" {
			hosts[host] = true
		}
	} else {
		// Default OpenAI host
		hosts["api.openai.com"] = true
	}

	// Extract from Anthropic
	if cfg.Providers.Anthropic.BaseURL != "" {
		if host := extractHost(cfg.Providers.Anthropic.BaseURL); host != "" {
			hosts[host] = true
		}
	} else {
		// Default Anthropic host
		hosts["api.anthropic.com"] = true
	}

	// Extract from Azure OpenAI
	if cfg.Providers.AzureOpenAI.Endpoint != "" {
		if host := extractHost(cfg.Providers.AzureOpenAI.Endpoint); host != "" {
			hosts[host] = true
		}
	}

	// Extract from Bedrock (if custom endpoint)
	if cfg.Providers.Bedrock.Region != "" {
		// Standard bedrock hostnames
		hosts[fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", cfg.Providers.Bedrock.Region)] = true
	}

	// Convert to slice
	result := make([]string, 0, len(hosts))
	for host := range hosts {
		result = append(result, host)
	}

	return result
}

// extractHost extracts hostname from a URL string
func extractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	// Handle URLs without scheme
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return parsed.Hostname()
}
