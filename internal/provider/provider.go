// Package provider defines the inference backend interface and concrete implementations
// for Anthropic, OpenAI, and Ollama.
package provider

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role
	Content string
}

// Provider is the inference backend Kate thinks with.
// Complete sends the conversation history and returns Kate's next reply.
type Provider interface {
	Complete(ctx context.Context, messages []Message) (string, error)
	Model() string
}
