// Package agent wires together Kate's inference backend, her tools, and the
// harness that quantizes her actions to tick boundaries.
package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/allofher/agent-kate-reid/internal/harness"
	"github.com/allofher/agent-kate-reid/internal/provider"
	"github.com/allofher/agent-kate-reid/internal/tools"
)

// Kate is the agent. She holds the inference provider she thinks with, the
// tool registry she acts through, and the scheduler reports that wake her.
type Kate struct {
	Provider provider.Provider
	Registry *tools.Registry

	reports      <-chan harness.TickReport
	systemPrompt string
	history      []provider.Message
}

// New returns a Kate instance ready to join a session.
func New(p provider.Provider, registry *tools.Registry, reports <-chan harness.TickReport, systemPrompt string) *Kate {
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	return &Kate{
		Provider:     p,
		Registry:     registry,
		reports:      reports,
		systemPrompt: systemPrompt,
	}
}

// maxToolRounds bounds consecutive tool-call rounds within one window so a
// confused model can't spin without ever yielding to the scheduler.
const maxToolRounds = 8

// Run is Kate's life in the session: take a turn, act through tools, then
// wait for the next tick report and go again. It returns when ctx is
// cancelled.
func (k *Kate) Run(ctx context.Context) error {
	k.history = []provider.Message{
		{Role: provider.RoleSystem, Content: k.systemPrompt},
		{Role: provider.RoleUser, Content: "The session is live. Listen to what's playing, then join in when you're ready."},
	}

	for {
		if err := k.takeTurns(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Inference or dispatch failure: log and wait for the next tick
			// rather than dying mid-session.
			log.Printf("kate: turn error: %v", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case report := <-k.reports:
			k.history = append(k.history, provider.Message{
				Role:    provider.RoleUser,
				Content: formatTickReport(report),
			})
		}
	}
}

// takeTurns lets Kate act repeatedly until she stops calling tools (or hits
// the round cap), stacking as many queued changes in this window as she likes.
func (k *Kate) takeTurns(ctx context.Context) error {
	for round := 0; round < maxToolRounds; round++ {
		resp, err := k.Provider.Complete(ctx, k.history, k.Registry.Defs())
		if err != nil {
			return err
		}

		k.history = append(k.history, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})
		if resp.Text != "" {
			log.Printf("kate: %s", resp.Text)
		}

		if len(resp.ToolCalls) == 0 {
			return nil // Kate is done for this window
		}

		results := make([]provider.ToolResult, 0, len(resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			log.Printf("kate → %s(%s)", call.Name, compact(string(call.Args)))
			results = append(results, k.Registry.Dispatch(ctx, call))
		}
		k.history = append(k.history, provider.Message{
			Role:        provider.RoleUser,
			ToolResults: results,
		})
	}
	return nil
}

// formatTickReport renders a scheduler report as Kate's wake-up message.
func formatTickReport(r harness.TickReport) string {
	var sb strings.Builder
	sb.WriteString("[tick report]\n")

	if len(r.Applied) == 0 {
		sb.WriteString("No changes were queued this window.\n")
	}
	for _, res := range r.Applied {
		switch {
		case res.Err != nil:
			fmt.Fprintf(&sb, "FAILED %s %s: %v\n", res.Op.Kind, res.Op.Channel, res.Err)
		case res.State.Error != nil:
			fmt.Fprintf(&sb, "applied %s %s — but Strudel reports an error: %s\n", res.Op.Kind, res.Op.Channel, *res.State.Error)
		default:
			fmt.Fprintf(&sb, "applied %s %s\n", res.Op.Kind, res.Op.Channel)
		}
	}

	fmt.Fprintf(&sb, "playing: %t\n", r.State.Playing)
	if r.State.Error != nil {
		fmt.Fprintf(&sb, "session error: %s\n", *r.State.Error)
	}
	if r.State.Code != "" {
		fmt.Fprintf(&sb, "current session code:\n%s", r.State.Code)
	} else {
		sb.WriteString("the session is silent")
	}
	return sb.String()
}

// compact trims tool-call args for one-line logging.
func compact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}
