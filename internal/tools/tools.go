// Package tools implements Kate's Strudel-specific tool set.
// Each function maps directly to a repl-control operation.
package tools

import (
	"context"
	"fmt"

	"github.com/allofher/agent-kate-reid/internal/strudel"
)

// Registry holds all tools wired to a Strudel client.
type Registry struct {
	client *strudel.Client
}

// New returns a Registry backed by the given Strudel client.
func New(client *strudel.Client) *Registry {
	return &Registry{client: client}
}

// EvalPattern updates a named channel with new Strudel code and evaluates it.
// All of Kate's active channels are re-evaluated together as concurrent $: blocks.
func (r *Registry) EvalPattern(ctx context.Context, channel, code string) (strudel.State, error) {
	if channel == "" {
		return strudel.State{}, fmt.Errorf("channel name required")
	}
	if code == "" {
		return strudel.State{}, fmt.Errorf("code required")
	}
	return r.client.EvalPattern(ctx, channel, code)
}

// StopChannel silences a single named channel without affecting others.
func (r *Registry) StopChannel(ctx context.Context, channel string) (strudel.State, error) {
	if channel == "" {
		return strudel.State{}, fmt.Errorf("channel name required")
	}
	return r.client.StopChannel(ctx, channel)
}

// StopAll silences everything Kate is playing and clears her channel state.
func (r *Registry) StopAll(ctx context.Context) (strudel.State, error) {
	return r.client.StopAll(ctx)
}

// GetSessionState returns the current REPL state, including all code currently
// in the editor (Kate's patterns and whatever else is loaded).
func (r *Registry) GetSessionState(ctx context.Context) (strudel.State, error) {
	return r.client.GetState(ctx)
}

// SetCPS sets the global tempo in cycles per second.
// Common values: 0.5 = 120bpm, 0.75 = 180bpm, 1.0 = 240bpm.
func (r *Registry) SetCPS(ctx context.Context, cps float64) (strudel.State, error) {
	if cps <= 0 {
		return strudel.State{}, fmt.Errorf("cps must be positive")
	}
	return r.client.SetCPS(ctx, cps)
}
