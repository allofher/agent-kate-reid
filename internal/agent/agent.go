// Package agent wires together Kate's inference backend and her Strudel tools.
package agent

import (
	"github.com/allofher/agent-kate-reid/internal/provider"
	"github.com/allofher/agent-kate-reid/internal/strudel"
)

// Kate is the agent. She holds a reference to the Strudel client she plays through
// and the inference provider she thinks with.
type Kate struct {
	Strudel  *strudel.Client
	Provider provider.Provider
}

// New returns a Kate instance ready to join a session.
func New(strudelClient *strudel.Client, p provider.Provider) *Kate {
	return &Kate{Strudel: strudelClient, Provider: p}
}
