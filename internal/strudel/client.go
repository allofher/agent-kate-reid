// Package strudel is a client for the repl-control WebSocket bridge.
// The bridge lives in this repo (bridge/) and serves both the WebSocket hub
// and the Strudel browser page on http://localhost:8081 — start it with
// `npm start` in bridge/.
package strudel

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

const defaultURL = "ws://localhost:8081"

// Client connects to the Strudel repl-control WebSocket server and maintains
// Kate's named channel state.
type Client struct {
	url string

	connMu sync.Mutex
	conn   *websocket.Conn

	// pending maps requestId → channel that will receive the State response.
	pendingMu sync.Mutex
	pending   map[string]chan result

	reqSeq atomic.Uint64

	// channels is Kate's current set of named pattern blocks.
	chanMu   sync.Mutex
	channels map[string]string

	// cps is stored separately so it's always prepended first.
	cps *float64
}

type result struct {
	state State
	err   error
}

// New returns a Client pointed at the given repl-control WebSocket URL.
// If url is empty it defaults to ws://localhost:8081.
func New(url string) *Client {
	if url == "" {
		url = defaultURL
	}
	// Accept http:// URLs as a convenience and convert them.
	url = strings.Replace(url, "http://", "ws://", 1)
	url = strings.Replace(url, "https://", "wss://", 1)
	return &Client{
		url:      url,
		pending:  make(map[string]chan result),
		channels: make(map[string]string),
	}
}

// Connect opens the WebSocket connection, sends the cli handshake, and starts
// the background read loop. It must be called before any tool operations.
func (c *Client) Connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("strudel: connect to %s: %w", c.url, err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	if err := c.write(wireMsg{Type: msgHandshake, ClientType: "cli"}); err != nil {
		return fmt.Errorf("strudel: handshake: %w", err)
	}

	go c.readLoop()
	return nil
}

// Close shuts down the WebSocket connection.
func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// EvalPattern updates channel ch to code, rebuilds Kate's combined pattern,
// and evaluates the result in the REPL.
func (c *Client) EvalPattern(ctx context.Context, ch, code string) (State, error) {
	c.chanMu.Lock()
	c.channels[ch] = code
	combined := c.buildCode()
	c.chanMu.Unlock()

	return c.command(ctx, actionEvaluate, map[string]any{"code": combined})
}

// StopChannel removes channel ch from Kate's active set and re-evaluates the
// remaining channels. If no channels remain it sends a stop command.
func (c *Client) StopChannel(ctx context.Context, ch string) (State, error) {
	c.chanMu.Lock()
	delete(c.channels, ch)
	combined := c.buildCode()
	c.chanMu.Unlock()

	if combined == "" {
		return c.command(ctx, actionStop, nil)
	}
	return c.command(ctx, actionEvaluate, map[string]any{"code": combined})
}

// StopAll silences everything Kate is running and clears her channel state.
func (c *Client) StopAll(ctx context.Context) (State, error) {
	c.chanMu.Lock()
	c.channels = make(map[string]string)
	c.cps = nil
	c.chanMu.Unlock()

	return c.command(ctx, actionStop, nil)
}

// GetState returns the current REPL state without changing anything.
func (c *Client) GetState(ctx context.Context) (State, error) {
	return c.command(ctx, actionGetState, nil)
}

// SetCPS sets the global cycles-per-second tempo and re-evaluates.
// 0.5 cps = 120 bpm. The setCps() call is always prepended to Kate's code.
func (c *Client) SetCPS(ctx context.Context, cps float64) (State, error) {
	c.chanMu.Lock()
	c.cps = &cps
	combined := c.buildCode()
	c.chanMu.Unlock()

	return c.command(ctx, actionEvaluate, map[string]any{"code": combined})
}

// buildCode assembles Kate's current state into a single Strudel code block.
// Channel order is sorted by name for deterministic output.
// Must be called with chanMu held.
func (c *Client) buildCode() string {
	var sb strings.Builder

	if c.cps != nil {
		fmt.Fprintf(&sb, "setCps(%g)\n", *c.cps)
	}

	// Sort channel names so the combined code is deterministic.
	names := make([]string, 0, len(c.channels))
	for name := range c.channels {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(&sb, "$: %s // %s\n", c.channels[name], name)
	}

	return strings.TrimSpace(sb.String())
}

// command sends a control message and waits for the matching state response.
func (c *Client) command(ctx context.Context, action string, payload map[string]any) (State, error) {
	id := fmt.Sprintf("req_%d", c.reqSeq.Add(1))
	ch := make(chan result, 1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	msg := wireMsg{
		Type:      msgControl,
		Action:    action,
		RequestID: id,
	}
	if payload != nil {
		msg.Payload = payload
	}

	if err := c.write(msg); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return State{}, fmt.Errorf("strudel: send %s: %w", action, err)
	}

	select {
	case r := <-ch:
		return r.state, r.err
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return State{}, ctx.Err()
	}
}

func (c *Client) write(msg wireMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) readLoop() {
	for {
		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		_, data, err := conn.ReadMessage()
		if err != nil {
			// Connection closed — drain all pending requests.
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				ch <- result{err: fmt.Errorf("strudel: connection closed: %w", err)}
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			return
		}

		var m wireMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		switch m.Type {
		case msgState:
			state := stateFromPayload(m.Payload)
			if m.RequestID != "" {
				c.pendingMu.Lock()
				if ch, ok := c.pending[m.RequestID]; ok {
					ch <- result{state: state}
					delete(c.pending, m.RequestID)
				}
				c.pendingMu.Unlock()
			}

		case msgError:
			if m.RequestID != "" {
				c.pendingMu.Lock()
				if ch, ok := c.pending[m.RequestID]; ok {
					ch <- result{err: fmt.Errorf("strudel: %s", m.Error)}
					delete(c.pending, m.RequestID)
				}
				c.pendingMu.Unlock()
			}
		}
	}
}

func stateFromPayload(p map[string]any) State {
	s := State{}
	if v, ok := p["code"].(string); ok {
		s.Code = v
	}
	if v, ok := p["playing"].(bool); ok {
		s.Playing = v
	}
	if v, ok := p["error"].(string); ok {
		s.Error = &v
	}
	if v, ok := p["timestamp"].(float64); ok {
		s.Timestamp = int64(v)
	}
	return s
}
