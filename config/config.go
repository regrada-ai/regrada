package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RegradaConfig represents the complete configuration for a Regrada project.
// It is persisted as .regrada.yaml in the project root.
type RegradaConfig struct {
	Version  string         `yaml:"version"`
	Project  string         `yaml:"project"`
	Env      string         `yaml:"env,omitempty"`
	Provider ProviderConfig `yaml:"provider"`

	// Deprecated fields (kept for backward compatibility)
	Capture CaptureConfig `yaml:"capture,omitempty"`
	Evals   EvalsConfig   `yaml:"evals,omitempty"`
	Gate    GateConfig    `yaml:"gate,omitempty"`
	Output  OutputConfig  `yaml:"output,omitempty"`
}

// ProviderConfig defines the LLM provider settings for evaluations.
// Supported providers: openai, anthropic, azure-openai, custom.
type ProviderConfig struct {
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url,omitempty"`
	Model   string `yaml:"model,omitempty"`
}

// CaptureConfig controls what data is captured during LLM tracing (DEPRECATED).
type CaptureConfig struct {
	Requests  bool `yaml:"requests"`
	Responses bool `yaml:"responses"`
	Traces    bool `yaml:"traces"`
	Latency   bool `yaml:"latency"`
}

// EvalsConfig defines settings for running evaluations (DEPRECATED - most fields unused).
type EvalsConfig struct {
	Path       string   `yaml:"path"`
	Types      []string `yaml:"types,omitempty"`       // Not used
	Timeout    string   `yaml:"timeout,omitempty"`     // Not implemented
	Concurrent int      `yaml:"concurrent,omitempty"`  // Not implemented
}

// GateConfig defines quality gate thresholds for CI/CD integration (DEPRECATED - use --ci flag).
type GateConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Threshold float64 `yaml:"threshold,omitempty"`  // Not used
	FailOn    string  `yaml:"fail_on,omitempty"`    // Not used
}

// OutputConfig controls the format and verbosity of command output (DEPRECATED - use --output flag).
type OutputConfig struct {
	Format  string `yaml:"format,omitempty"`   // Not used
	Verbose bool   `yaml:"verbose,omitempty"`  // Not used
}

// Load reads and parses a Regrada configuration file.
func Load(path string) (*RegradaConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config RegradaConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Defaults returns a default configuration for a new project.
func Defaults(projectName string) *RegradaConfig {
	return &RegradaConfig{
		Version: "1",
		Project: projectName,
		Env:     "local",
		Provider: ProviderConfig{
			Type:  "openai",
			Model: "gpt-4o",
		},
		// Deprecated fields with default values for backward compatibility
		Capture: CaptureConfig{
			Requests:  true,
			Responses: true,
			Traces:    true,
			Latency:   true,
		},
		Evals: EvalsConfig{
			Path: "evals",
		},
	}
}

// Validate checks that the configuration is valid and complete.
// It also prints warnings for deprecated fields.
func Validate(cfg *RegradaConfig) error {
	if cfg.Version == "" {
		return fmt.Errorf("config version is required")
	}
	if cfg.Project == "" {
		return fmt.Errorf("project name is required")
	}
	if cfg.Provider.Type == "" {
		return fmt.Errorf("provider type is required")
	}

	// Validate provider type
	validProviders := map[string]bool{
		"openai":       true,
		"anthropic":    true,
		"azure-openai": true,
		"custom":       true,
	}
	if !validProviders[cfg.Provider.Type] {
		return fmt.Errorf("invalid provider type: %s (must be one of: openai, anthropic, azure-openai, custom)", cfg.Provider.Type)
	}

	// Print warnings for deprecated fields
	if cfg.Capture.Requests || cfg.Capture.Responses || cfg.Capture.Traces || cfg.Capture.Latency {
		fmt.Fprintf(os.Stderr, "Warning: 'capture' config is deprecated (all data is now captured by default)\n")
	}
	if len(cfg.Evals.Types) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: 'evals.types' config is deprecated and ignored\n")
	}
	if cfg.Evals.Timeout != "" {
		fmt.Fprintf(os.Stderr, "Warning: 'evals.timeout' config is deprecated and ignored\n")
	}
	if cfg.Evals.Concurrent > 0 {
		fmt.Fprintf(os.Stderr, "Warning: 'evals.concurrent' config is deprecated and ignored\n")
	}
	if cfg.Gate.Enabled {
		fmt.Fprintf(os.Stderr, "Warning: 'gate' config is deprecated, use --ci flag instead\n")
	}
	if cfg.Output.Format != "" || cfg.Output.Verbose {
		fmt.Fprintf(os.Stderr, "Warning: 'output' config is deprecated, use --output flag instead\n")
	}

	return nil
}
