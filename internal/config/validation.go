package config

import (
	"errors"
	"fmt"
)

// validateConfig validates the configuration
func validateConfig(cfg *ProjectConfig) error {
	validators := []func(*ProjectConfig) error{
		validateVersion,
		validateCaptureMode,
		validateBaselineMode,
		validateProvider,
		validateProviderConfigs,
		validatePolicies,
	}

	for _, validator := range validators {
		if err := validator(cfg); err != nil {
			return err
		}
	}
	return nil
}

func validateVersion(cfg *ProjectConfig) error {
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported version %d", cfg.Version)
	}
	return nil
}

func validateCaptureMode(cfg *ProjectConfig) error {
	if cfg.Capture.Mode != "proxy" && cfg.Capture.Mode != "off" {
		return fmt.Errorf("capture.mode must be proxy or off")
	}
	return nil
}

func validateBaselineMode(cfg *ProjectConfig) error {
	if cfg.Baseline.Mode != "git" && cfg.Baseline.Mode != "local" {
		return fmt.Errorf("baseline.mode must be git or local")
	}
	return nil
}

func validateProvider(cfg *ProjectConfig) error {
	if cfg.Providers.Default != "" && !isValidProvider(cfg.Providers.Default) {
		return fmt.Errorf("providers.default must be one of openai, anthropic, azure_openai, bedrock, mock")
	}
	return nil
}

func validatePolicies(cfg *ProjectConfig) error {
	for i := range cfg.Policies {
		policy := &cfg.Policies[i]

		if policy.ID == "" {
			return fmt.Errorf("policies[%d].id is required", i)
		}

		if err := validatePolicySeverity(policy, i); err != nil {
			return err
		}

		if err := validatePolicyCheckDefaults(policy); err != nil {
			return err
		}

		if err := validatePolicyCheck(policy.Check); err != nil {
			return fmt.Errorf("policies[%d].check: %w", i, err)
		}
	}
	return nil
}

func validatePolicySeverity(policy *Policy, index int) error {
	if policy.Severity == "" {
		policy.Severity = "error"
		return nil
	}

	if policy.Severity != "error" && policy.Severity != "warn" {
		return fmt.Errorf("policies[%d].severity must be error or warn", index)
	}
	return nil
}

func validatePolicyCheckDefaults(policy *Policy) error {
	if policy.Check.Type == "json_valid" && policy.Check.Extractor == "" {
		policy.Check.Extractor = "assistant_text"
	}
	return nil
}

func validatePolicyCheck(check PolicyCheck) error {
	if check.Type == "" {
		return errors.New("type is required")
	}

	validator, exists := policyCheckValidators[check.Type]
	if !exists {
		return fmt.Errorf("unsupported check type %q", check.Type)
	}

	return validator(check)
}

// policyCheckValidators maps check types to their validation functions
var policyCheckValidators = map[string]func(PolicyCheck) error{
	"json_valid":        validateJSONValid,
	"json_schema":       validateJSONSchema,
	"text_contains":     validateTextContains,
	"text_not_contains": validateTextContains,
	"pii_leak":          validatePIILeak,
	"variance":          validateVariance,
	"refusal_rate":      validateRefusalRate,
	"latency":           validateLatency,
	"assertions":        validateAssertions,
}

func validateJSONValid(check PolicyCheck) error {
	if check.Extractor == "" {
		check.Extractor = "assistant_text"
	}
	return nil
}

func validateJSONSchema(check PolicyCheck) error {
	if check.Schema == "" {
		return errors.New("schema is required")
	}
	return nil
}

func validateTextContains(check PolicyCheck) error {
	if len(check.Phrases) == 0 {
		return errors.New("phrases is required")
	}
	return nil
}

func validatePIILeak(check PolicyCheck) error {
	if check.Detector == "" {
		return errors.New("detector is required")
	}
	return nil
}

func validateVariance(check PolicyCheck) error {
	if check.Metric == "" {
		return errors.New("metric is required")
	}
	return nil
}

func validateRefusalRate(check PolicyCheck) error {
	if check.Max == nil && check.MaxDelta == nil {
		return errors.New("max or max_delta is required")
	}
	return nil
}

func validateLatency(check PolicyCheck) error {
	if check.LatencyP95 == nil {
		return errors.New("p95_ms is required")
	}
	return nil
}

func validateAssertions(check PolicyCheck) error {
	if check.MinPassRate == nil {
		return errors.New("min_pass_rate is required")
	}
	return nil
}

func isValidProvider(value string) bool {
	validProviders := map[string]bool{
		"openai":       true,
		"anthropic":    true,
		"azure_openai": true,
		"bedrock":      true,
		"mock":         true,
	}
	return validProviders[value]
}

func validateProviderConfigs(cfg *ProjectConfig) error {
	validators := []func(*ProvidersConfig) error{
		validateOpenAIConfig,
		validateAnthropicConfig,
		validateAzureOpenAIConfig,
		validateBedrockConfig,
	}

	for _, validator := range validators {
		if err := validator(&cfg.Providers); err != nil {
			return err
		}
	}
	return nil
}

func validateOpenAIConfig(providers *ProvidersConfig) error {
	if !isProviderConfigSet(
		providers.OpenAI.APIKeyEnv,
		providers.OpenAI.APIKey,
		providers.OpenAI.BaseURLEnv,
		providers.OpenAI.BaseURL,
		providers.OpenAI.Model,
	) {
		return nil
	}

	if providers.OpenAI.APIKeyEnv == "" && providers.OpenAI.APIKey == "" {
		return fmt.Errorf("providers.openai: either api_key_env or api_key must be provided")
	}
	return nil
}

func validateAnthropicConfig(providers *ProvidersConfig) error {
	if !isProviderConfigSet(
		providers.Anthropic.APIKeyEnv,
		providers.Anthropic.APIKey,
		providers.Anthropic.BaseURLEnv,
		providers.Anthropic.BaseURL,
		providers.Anthropic.Model,
	) {
		return nil
	}

	if providers.Anthropic.APIKeyEnv == "" && providers.Anthropic.APIKey == "" {
		return fmt.Errorf("providers.anthropic: either api_key_env or api_key must be provided")
	}
	return nil
}

func validateAzureOpenAIConfig(providers *ProvidersConfig) error {
	if !isProviderConfigSet(
		providers.AzureOpenAI.APIKeyEnv,
		providers.AzureOpenAI.APIKey,
		providers.AzureOpenAI.EndpointEnv,
		providers.AzureOpenAI.Endpoint,
		providers.AzureOpenAI.APIVersion,
		providers.AzureOpenAI.Deployment,
	) {
		return nil
	}

	if providers.AzureOpenAI.APIKeyEnv == "" && providers.AzureOpenAI.APIKey == "" {
		return fmt.Errorf("providers.azure_openai: either api_key_env or api_key must be provided")
	}
	if providers.AzureOpenAI.EndpointEnv == "" && providers.AzureOpenAI.Endpoint == "" {
		return fmt.Errorf("providers.azure_openai: either endpoint_env or endpoint must be provided")
	}
	return nil
}

func validateBedrockConfig(providers *ProvidersConfig) error {
	if !isProviderConfigSet(
		providers.Bedrock.RegionEnv,
		providers.Bedrock.Region,
		providers.Bedrock.AccessKeyEnv,
		providers.Bedrock.AccessKey,
		providers.Bedrock.SecretKeyEnv,
		providers.Bedrock.SecretKey,
		providers.Bedrock.ModelID,
	) {
		return nil
	}

	if providers.Bedrock.RegionEnv == "" && providers.Bedrock.Region == "" {
		return fmt.Errorf("providers.bedrock: either region_env or region must be provided")
	}
	if providers.Bedrock.AccessKeyEnv == "" && providers.Bedrock.AccessKey == "" {
		return fmt.Errorf("providers.bedrock: either access_key_env or access_key must be provided")
	}
	if providers.Bedrock.SecretKeyEnv == "" && providers.Bedrock.SecretKey == "" {
		return fmt.Errorf("providers.bedrock: either secret_key_env or secret_key must be provided")
	}
	return nil
}

// isProviderConfigSet checks if any fields in the provider config are set
func isProviderConfigSet(fields ...string) bool {
	for _, field := range fields {
		if field != "" {
			return true
		}
	}
	return false
}
