package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
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

func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	return completeChat(ctx, p.client, p.model, "openai", messages, tools)
}

// completeChat is the Chat Completions code path shared by OpenAIProvider and
// OllamaProvider (Ollama exposes an OpenAI-compatible endpoint).
func completeChat(ctx context.Context, client *openai.Client, model, label string, messages []Message, tools []ToolDef) (Response, error) {
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    model,
		Messages: buildOpenAIMessages(messages),
		Tools:    buildOpenAITools(tools),
	})
	if err != nil {
		return Response{}, fmt.Errorf("%s: %w", label, err)
	}
	if len(resp.Choices) == 0 {
		return Response{}, fmt.Errorf("%s: empty response", label)
	}

	msg := resp.Choices[0].Message
	out := Response{Text: msg.Content}
	for _, tc := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(tc.Function.Arguments),
		})
	}
	return out, nil
}

func buildOpenAITools(tools []ToolDef) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, t := range tools {
		out = append(out, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  shared.FunctionParameters(t.InputSchema),
			},
		})
	}
	return out
}

func buildOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	var out []openai.ChatCompletionMessageParamUnion
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			out = append(out, openai.SystemMessage(m.Content))
		case RoleUser:
			// Tool results map to dedicated "tool" role messages.
			for _, r := range m.ToolResults {
				content := r.Content
				if r.IsError {
					content = "ERROR: " + content
				}
				out = append(out, openai.ToolMessage(content, r.ToolCallID))
			}
			if m.Content != "" {
				out = append(out, openai.UserMessage(m.Content))
			}
		case RoleAssistant:
			if len(m.ToolCalls) == 0 {
				out = append(out, openai.AssistantMessage(m.Content))
				continue
			}
			assistant := openai.ChatCompletionAssistantMessageParam{}
			if m.Content != "" {
				assistant.Content.OfString = openai.String(m.Content)
			}
			for _, c := range m.ToolCalls {
				assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallParam{
					ID: c.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      c.Name,
						Arguments: string(c.Args),
					},
				})
			}
			out = append(out, openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
		}
	}
	return out
}
