// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

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

// EvalsConfig defines settings for running evaluations.
type EvalsConfig struct {
	Path       string   `yaml:"path"`
	Types      []string `yaml:"types,omitempty"`
	Timeout    string   `yaml:"timeout,omitempty"`
	Concurrent int      `yaml:"concurrent,omitempty"`
}

// GateConfig defines quality gate thresholds for CI/CD integration.
type GateConfig struct {
	Enabled   bool    `yaml:"enabled"`
	Threshold float64 `yaml:"threshold,omitempty"`
	FailOn    string  `yaml:"fail_on,omitempty"` // Options: any-failure, regression, threshold
}

// OutputConfig controls the format and verbosity of command output.
type OutputConfig struct {
	Format  string `yaml:"format,omitempty"` // Options: text, json, github
	Verbose bool   `yaml:"verbose,omitempty"`
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
		Capture: CaptureConfig{
			Requests:  true,
			Responses: true,
			Traces:    true,
			Latency:   true,
		},
		Evals: EvalsConfig{
			Path:       "evals",
			Types:      []string{"semantic", "exact", "llm-judge"},
			Timeout:    "30s",
			Concurrent: 5,
		},
		Gate: GateConfig{
			Enabled:   true,
			Threshold: 0.85,
			FailOn:    "any-failure",
		},
		Output: OutputConfig{
			Format:  "github",
			Verbose: false,
		},
	}
}

// Validate checks that the configuration is valid and complete.
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

	// Validate gate fail_on option
	if cfg.Gate.FailOn != "" {
		validFailOn := map[string]bool{
			"any-failure": true,
			"regression":  true,
			"threshold":   true,
		}
		if !validFailOn[cfg.Gate.FailOn] {
			fmt.Fprintf(os.Stderr, "Warning: invalid gate.fail_on value '%s' (valid options: any-failure, regression, threshold)\n", cfg.Gate.FailOn)
		}
	}

	// Validate output format
	if cfg.Output.Format != "" {
		validFormats := map[string]bool{
			"text":   true,
			"json":   true,
			"github": true,
		}
		if !validFormats[cfg.Output.Format] {
			fmt.Fprintf(os.Stderr, "Warning: invalid output.format value '%s' (valid options: text, json, github)\n", cfg.Output.Format)
		}
	}

	return nil
}
