package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigYML  = "regrada.yml"
	defaultConfigYAML = "regrada.yaml"
)

// LoadProjectConfig loads and validates a project configuration from a file
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	configPath, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg, err := parseConfig(data)
	if err != nil {
		return nil, err
	}

	if err := applyDefaults(cfg, configPath); err != nil {
		return nil, err
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseConfig parses YAML data into a ProjectConfig
func parseConfig(data []byte) (*ProjectConfig, error) {
	var cfg ProjectConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// resolveConfigPath determines the config file path to use
func resolveConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	// Try default file names in order
	defaultPaths := []string{defaultConfigYML, defaultConfigYAML}
	for _, p := range defaultPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("config not found (looked for %s or %s)", defaultConfigYML, defaultConfigYAML)
}

// DefaultConfig creates a new configuration with all defaults applied
func DefaultConfig(projectName string) ProjectConfig {
	cfg := ProjectConfig{
		Version: 1,
		Project: ProjectMeta{Name: projectName, Root: "."},
	}
	_ = applyDefaults(&cfg, filepath.Join(projectName, defaultConfigYML))
	return cfg
}
