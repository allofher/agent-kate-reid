package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Provider         string          `json:"provider"` // "anthropic" | "openai" | "ollama"
	Model            string          `json:"model"`
	StrudelURL       string          `json:"strudel_url"`
	SystemPromptPath string          `json:"system_prompt_path"` // optional override of the embedded prompt
	Harness          HarnessConfig   `json:"harness"`
	Anthropic        AnthropicConfig `json:"anthropic"`
	OpenAI           OpenAIConfig    `json:"openai"`
	Ollama           OllamaConfig    `json:"ollama"`
}

// HarnessConfig controls how queued changes are applied to Strudel.
type HarnessConfig struct {
	TickMode    string  `json:"tick_mode"`    // "cycles" | "wallclock"
	TickCycles  float64 `json:"tick_cycles"`  // cycles mode: apply every N cycles
	TickSeconds float64 `json:"tick_seconds"` // wallclock mode: apply every N seconds
	DefaultCPS  float64 `json:"default_cps"`  // cycles mode: tempo anchor before Kate sets one
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
		// The bridge (bridge/server.mjs) — serves the WebSocket hub and the
		// Strudel browser page on the same port.
		c.StrudelURL = "ws://localhost:8081"
	}
	if c.Provider == "ollama" && c.Ollama.BaseURL == "" {
		c.Ollama.BaseURL = "http://localhost:11434"
	}
	return c.Harness.validate()
}

func (h *HarnessConfig) validate() error {
	if h.TickMode == "" {
		h.TickMode = "cycles"
	}
	switch h.TickMode {
	case "cycles":
		if h.TickCycles == 0 {
			h.TickCycles = 4
		}
		if h.TickCycles < 0 {
			return fmt.Errorf("harness.tick_cycles must be positive")
		}
		if h.DefaultCPS == 0 {
			h.DefaultCPS = 0.5
		}
		if h.DefaultCPS < 0 {
			return fmt.Errorf("harness.default_cps must be positive")
		}
	case "wallclock":
		if h.TickSeconds == 0 {
			h.TickSeconds = 8
		}
		if h.TickSeconds < 0 {
			return fmt.Errorf("harness.tick_seconds must be positive")
		}
	default:
		return fmt.Errorf("unknown harness.tick_mode %q; must be cycles or wallclock", h.TickMode)
	}
	return nil
}
