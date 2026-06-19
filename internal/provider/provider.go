// Package provider defines the inference backend interface and concrete implementations
// for Anthropic, OpenAI, and Ollama.
package provider

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ToolDef describes a tool the model may call. InputSchema is a JSON Schema
// object ({"type": "object", "properties": ..., "required": ...}).
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is the model asking for a tool to be executed.
type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// ToolResult is the outcome of executing a ToolCall, fed back to the model.
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// Message is a single turn in a conversation. Assistant turns may carry
// ToolCalls; user turns may carry ToolResults. Each provider maps these onto
// its native wire format.
type Message struct {
	Role        Role
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
}

// Response is the model's reply for one turn. ToolCalls is non-empty when the
// model wants tools executed before it continues.
type Response struct {
	Text      string
	ToolCalls []ToolCall
}

// Provider is the inference backend Kate thinks with.
// Complete sends the conversation history and available tools, returning
// Kate's next reply.
type Provider interface {
	Complete(ctx context.Context, messages []Message, tools []ToolDef) (Response, error)
	Model() string
}
