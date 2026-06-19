package strudel

// Wire types for the repl-control WebSocket protocol.
// The server side lives in this repo: bridge/server.mjs (relay hub) and
// bridge/public/index.html (browser REPL). Spec: docs/tools.md.

const (
	msgHandshake = "handshake"
	msgControl   = "control"
	msgState     = "state"
	msgError     = "error"
)

const (
	actionEvaluate = "evaluate"
	actionPlay     = "play"
	actionStop     = "stop"
	actionGetState = "getState"
)

type wireMsg struct {
	Type       string         `json:"type"`
	ClientType string         `json:"clientType,omitempty"`
	Action     string         `json:"action,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	RequestID  string         `json:"requestId,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// State is the REPL state returned by the browser after every command.
type State struct {
	Code      string  `json:"code"`
	Playing   bool    `json:"playing"`
	Error     *string `json:"error"`
	Timestamp int64   `json:"timestamp"`
}
