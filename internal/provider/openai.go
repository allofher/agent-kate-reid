package provider

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAIProvider calls the OpenAI Chat Completions API.
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAI(apiKey, model string) *OpenAIProvider {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIProvider{client: &c, model: model}
}

func (p *OpenAIProvider) Model() string { return p.model }

func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: buildOpenAIMessages(messages),
	})
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return resp.Choices[0].Message.Content, nil
}

// buildOpenAIMessages is shared with OllamaProvider.
func buildOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			out = append(out, openai.SystemMessage(m.Content))
		case RoleUser:
			out = append(out, openai.UserMessage(m.Content))
		case RoleAssistant:
			out = append(out, openai.AssistantMessage(m.Content))
		}
	}
	return out
}
