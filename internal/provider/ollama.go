package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OllamaProvider calls a local Ollama instance via its OpenAI-compatible /v1 endpoint.
type OllamaProvider struct {
	client *openai.Client
	model  string
}

func NewOllama(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if !strings.HasSuffix(baseURL, "/v1") && !strings.HasSuffix(baseURL, "/v1/") {
		baseURL = strings.TrimRight(baseURL, "/") + "/v1/"
	}
	c := openai.NewClient(
		option.WithAPIKey("ollama"), // Ollama ignores the key; value must be non-empty
		option.WithBaseURL(baseURL),
	)
	return &OllamaProvider{client: &c, model: model}
}

func (p *OllamaProvider) Model() string { return p.model }

func (p *OllamaProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: buildOpenAIMessages(messages),
	})
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("ollama: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}
