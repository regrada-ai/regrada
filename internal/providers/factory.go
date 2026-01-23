package providers

import (
	"fmt"

	"github.com/regrada-ai/regrada/internal/config"
)

func Resolve(cfg *config.ProjectConfig) (Provider, error) {
	switch cfg.Providers.Default {
	case "mock":
		return NewMockProvider(), nil
	case "openai":
		return NewOpenAIProvider(cfg)
	case "anthropic", "azure_openai", "bedrock":
		return nil, fmt.Errorf("provider %q not implemented in skeleton", cfg.Providers.Default)
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Providers.Default)
	}
}
