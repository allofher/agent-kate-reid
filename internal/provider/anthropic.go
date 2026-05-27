package provider

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider calls the Anthropic Messages API.
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

func NewAnthropic(apiKey, model string) *AnthropicProvider {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{client: &c, model: model}
}

func (p *AnthropicProvider) Model() string { return p.model }

func (p *AnthropicProvider) Complete(ctx context.Context, messages []Message) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 4096,
		Messages:  buildAnthropicMessages(messages),
	}

	if sys := extractSystem(messages); sys != "" {
		params.System = []anthropic.TextBlockParam{{Text: sys}}
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return resp.Content[0].Text, nil
}

func buildAnthropicMessages(messages []Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range messages {
		switch m.Role {
		case RoleUser:
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case RoleAssistant:
			out = append(out, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		// RoleSystem is passed via MessageNewParams.System, not in the messages slice
		}
	}
	return out
}

func extractSystem(messages []Message) string {
	for _, m := range messages {
		if m.Role == RoleSystem {
			return m.Content
		}
	}
	return ""
}
