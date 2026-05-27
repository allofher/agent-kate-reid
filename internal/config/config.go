package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Provider   string          `json:"provider"`   // "anthropic" | "openai" | "ollama"
	Model      string          `json:"model"`
	StrudelURL string          `json:"strudel_url"`
	Anthropic  AnthropicConfig `json:"anthropic"`
	OpenAI     OpenAIConfig    `json:"openai"`
	Ollama     OllamaConfig    `json:"ollama"`
}

type AnthropicConfig struct {
	APIKey string `json:"api_key"`
}

type OpenAIConfig struct {
	APIKey string `json:"api_key"`
}

type OllamaConfig struct {
	BaseURL string `json:"base_url"` // defaults to http://localhost:11434
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return &cfg, cfg.validate()
}

func (c *Config) validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider is required (anthropic | openai | ollama)")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	switch c.Provider {
	case "anthropic", "openai", "ollama":
	default:
		return fmt.Errorf("unknown provider %q; must be anthropic, openai, or ollama", c.Provider)
	}
	if c.StrudelURL == "" {
		c.StrudelURL = "http://localhost:3000"
	}
	if c.Provider == "ollama" && c.Ollama.BaseURL == "" {
		c.Ollama.BaseURL = "http://localhost:11434"
	}
	return nil
}
