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

func (p *AnthropicProvider) Complete(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 4096,
		Messages:  buildAnthropicMessages(messages),
		Tools:     buildAnthropicTools(tools),
	}

	if sys := extractSystem(messages); sys != "" {
		params.System = []anthropic.TextBlockParam{{Text: sys}}
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic: %w", err)
	}

	var out Response
	for _, block := range resp.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			out.Text += variant.Text
		case anthropic.ToolUseBlock:
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:   variant.ID,
				Name: variant.Name,
				Args: variant.Input,
			})
		}
	}
	return out, nil
}

func buildAnthropicTools(tools []ToolDef) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		tool := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: t.InputSchema["properties"],
			},
		}
		if req, ok := t.InputSchema["required"].([]string); ok {
			tool.InputSchema.Required = req
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return out
}

func buildAnthropicMessages(messages []Message) []anthropic.MessageParam {
	var out []anthropic.MessageParam
	for _, m := range messages {
		switch m.Role {
		case RoleUser:
			var blocks []anthropic.ContentBlockParamUnion
			// Tool results must lead the user turn that answers a tool_use.
			for _, r := range m.ToolResults {
				blocks = append(blocks, anthropic.NewToolResultBlock(r.ToolCallID, r.Content, r.IsError))
			}
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			out = append(out, anthropic.NewUserMessage(blocks...))
		case RoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			for _, c := range m.ToolCalls {
				blocks = append(blocks, anthropic.NewToolUseBlock(c.ID, c.Args, c.Name))
			}
			out = append(out, anthropic.NewAssistantMessage(blocks...))
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
