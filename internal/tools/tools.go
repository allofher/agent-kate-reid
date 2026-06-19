// Package tools implements Kate's Strudel-specific tool set.
//
// Mutating tools (eval_pattern, stop_channel, stop_all, set_cps) do not touch
// the Strudel server directly: they enqueue operations on the harness queue,
// which the scheduler applies at the next tick boundary. get_session_state is
// read-only and executes immediately.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/allofher/agent-kate-reid/internal/harness"
	"github.com/allofher/agent-kate-reid/internal/provider"
	"github.com/allofher/agent-kate-reid/internal/strudel"
)

// Reader is the read-only slice of the Strudel client the registry uses.
type Reader interface {
	GetState(ctx context.Context) (strudel.State, error)
}

// Registry holds Kate's tools, wired to the harness queue and Strudel client.
type Registry struct {
	queue  *harness.Queue
	reader Reader
}

// New returns a Registry backed by the given queue and state reader.
func New(queue *harness.Queue, reader Reader) *Registry {
	return &Registry{queue: queue, reader: reader}
}

// Defs returns the tool definitions exposed to the inference provider.
func (r *Registry) Defs() []provider.ToolDef {
	return []provider.ToolDef{
		{
			Name: "eval_pattern",
			Description: "Write or replace the Strudel pattern on one of your named channels " +
				"(e.g. \"drums\", \"bass\", \"melody\"). Pass raw Strudel code for that single layer — " +
				"do not include $: or setCps(), the harness assembles those. The change is QUEUED and " +
				"applied at the next tick boundary so it lands musically; the result (including any " +
				"eval errors) arrives in your next tick report. You can queue several channels in one " +
				"turn; queueing the same channel twice keeps only the latest code.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"channel": map[string]any{
						"type":        "string",
						"description": "Your name for this layer, e.g. \"drums\", \"bass\", \"pads\"",
					},
					"code": map[string]any{
						"type":        "string",
						"description": "Strudel pattern code for this channel, e.g. s(\"bd hh sd hh\")",
					},
				},
				"required": []string{"channel", "code"},
			},
		},
		{
			Name: "stop_channel",
			Description: "Silence one of your channels at the next tick boundary, leaving the others " +
				"playing. Use when a layer has run its course or you want to thin the texture.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"channel": map[string]any{
						"type":        "string",
						"description": "The channel to silence",
					},
				},
				"required": []string{"channel"},
			},
		},
		{
			Name: "stop_all",
			Description: "Silence everything you are playing at the next tick boundary and clear any " +
				"changes you queued this window. Use sparingly — e.g. for a full break or to recover " +
				"from a mess.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name: "set_cps",
			Description: "Set the global tempo in cycles per second, applied at the next tick boundary. " +
				"0.5 cps = 120 bpm at 4 beats per cycle. Only change tempo deliberately — it affects " +
				"the whole band.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cps": map[string]any{
						"type":        "number",
						"description": "Cycles per second; must be positive. 0.5 = 120bpm, 0.75 = 180bpm",
					},
				},
				"required": []string{"cps"},
			},
		},
		{
			Name: "get_session_state",
			Description: "Read the current REPL state immediately (not queued): all code currently " +
				"playing — yours and the other players'. Use this to listen before you act: see what " +
				"notes, rhythms and density are in play so you can complement rather than clash.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// Dispatch executes (or enqueues) one tool call and returns its result.
func (r *Registry) Dispatch(ctx context.Context, call provider.ToolCall) provider.ToolResult {
	content, err := r.dispatch(ctx, call)
	if err != nil {
		return provider.ToolResult{ToolCallID: call.ID, Content: err.Error(), IsError: true}
	}
	return provider.ToolResult{ToolCallID: call.ID, Content: content}
}

func (r *Registry) dispatch(ctx context.Context, call provider.ToolCall) (string, error) {
	switch call.Name {
	case "eval_pattern":
		var args struct {
			Channel string `json:"channel"`
			Code    string `json:"code"`
		}
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return "", fmt.Errorf("eval_pattern: bad arguments: %w", err)
		}
		if args.Channel == "" {
			return "", fmt.Errorf("eval_pattern: channel name required")
		}
		if args.Code == "" {
			return "", fmt.Errorf("eval_pattern: code required")
		}
		r.queue.Enqueue(harness.Op{Kind: harness.OpEval, Channel: args.Channel, Code: args.Code})
		return fmt.Sprintf("queued: channel %q will be updated at the next tick", args.Channel), nil

	case "stop_channel":
		var args struct {
			Channel string `json:"channel"`
		}
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return "", fmt.Errorf("stop_channel: bad arguments: %w", err)
		}
		if args.Channel == "" {
			return "", fmt.Errorf("stop_channel: channel name required")
		}
		r.queue.Enqueue(harness.Op{Kind: harness.OpStopChannel, Channel: args.Channel})
		return fmt.Sprintf("queued: channel %q will be silenced at the next tick", args.Channel), nil

	case "stop_all":
		r.queue.Enqueue(harness.Op{Kind: harness.OpStopAll})
		return "queued: everything will be silenced at the next tick", nil

	case "set_cps":
		var args struct {
			CPS float64 `json:"cps"`
		}
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return "", fmt.Errorf("set_cps: bad arguments: %w", err)
		}
		if args.CPS <= 0 {
			return "", fmt.Errorf("set_cps: cps must be positive")
		}
		r.queue.Enqueue(harness.Op{Kind: harness.OpSetCPS, CPS: args.CPS})
		return fmt.Sprintf("queued: tempo will change to %g cps at the next tick", args.CPS), nil

	case "get_session_state":
		state, err := r.reader.GetState(ctx)
		if err != nil {
			return "", fmt.Errorf("get_session_state: %w", err)
		}
		data, err := json.Marshal(state)
		if err != nil {
			return "", fmt.Errorf("get_session_state: %w", err)
		}
		return string(data), nil

	default:
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
}
