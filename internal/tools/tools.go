// Package tools defines the Strudel-specific tool set Kate can call.
// Each function here maps directly to a capability on the local Strudel REPL.
package tools

import "github.com/allofher/agent-kate-reid/internal/strudel"

// Registry holds all tools wired to a Strudel client.
// Tools are registered here and exposed to the inference provider as function-call definitions.
type Registry struct {
	client *strudel.Client
}

// New returns an empty Registry backed by the given client.
func New(client *strudel.Client) *Registry {
	return &Registry{client: client}
}
