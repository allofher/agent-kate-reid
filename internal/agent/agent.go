// Package agent wires together Kate's inference backend and her Strudel tools.
package agent

import "github.com/allofher/agent-kate-reid/internal/strudel"

// Kate is the agent. She holds a reference to the Strudel client she plays through
// and the model backend she thinks with.
type Kate struct {
	Strudel *strudel.Client
	// TODO: inference provider (Anthropic / OpenAI / OpenRouter / Ollama)
}

// New returns a Kate instance ready to join a session.
func New(strudelClient *strudel.Client) *Kate {
	return &Kate{Strudel: strudelClient}
}
