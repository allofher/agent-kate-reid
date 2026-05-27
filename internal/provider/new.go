package provider

import (
	"fmt"

	"github.com/allofher/agent-kate-reid/internal/config"
)

// FromConfig constructs the correct Provider from the loaded config.
func FromConfig(cfg *config.Config) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		if cfg.Anthropic.APIKey == "" {
			return nil, fmt.Errorf("anthropic.api_key is required when provider is anthropic")
		}
		return NewAnthropic(cfg.Anthropic.APIKey, cfg.Model), nil
	case "openai":
		if cfg.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("openai.api_key is required when provider is openai")
		}
		return NewOpenAI(cfg.OpenAI.APIKey, cfg.Model), nil
	case "ollama":
		return NewOllama(cfg.Ollama.BaseURL, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
