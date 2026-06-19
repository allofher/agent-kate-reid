# Kate Reid — Tool Design & Strudel Architecture

## Tool design strategy

### The core question

The tools are not an abstraction over Strudel's functions — Strudel code IS the language
Kate writes. The tool surface covers the **runtime interface**: how Kate sends code, reads
session state, and navigates the ensemble. Two layers handle everything else.

---

### Layer 1 — Runtime tools (~5, small and stable)

These are what Kate calls at execution time. They map to server operations, not musical
concepts. Kate writes raw Strudel pattern strings herself; the tools handle the I/O.

| Tool | Signature | Purpose |
|------|-----------|---------|
| `eval_pattern` | `(channel string, code string)` | Send Strudel code to a named channel for immediate playback |
| `stop_channel` | `(channel string)` | Silence one of Kate's channels |
| `stop_all` | `()` | Panic button — silence everything Kate is running |
| `get_session_state` | `()` | Return all active channels and their current code (Kate's + other players') |
| `set_cps` | `(cps float)` | Set global cycles-per-second (Strudel's tempo primitive; 0.5 = 120bpm) |

No tools for `fast()`, `slow()`, `note()`, etc. — those are language, not runtime.

### Layer 2 — Strudel knowledge in the system prompt

The pattern vocabulary, idioms, and musical personality guidelines live in the system
prompt, not in tools. It is read once at session start, guides generation style, and
can be tuned without changing the tool interface.

---

### On audio / waveform input

**Raw waveform: no.** Enormous, unstructured, and LLMs have no capacity to reason
about spectral data symbolically.

The instinct behind the question is right — Kate needs to listen — but the better signal
is already available:

> **Kate's most valuable input is the other players' code, not their sound.**

`get_session_state()` returns what everyone else is currently running. From that Kate can:
- See what notes/sounds are in play and complement or contrast harmonically
- Read rhythmic structure and lock in or syncopate
- Gauge density and leave space accordingly

That is exactly what a session musician does: reads the chart, listens to the room.
The code IS the chart. Tempo and silence are readable from session state and `cps` —
raw audio is not needed for either.

---

## Strudel architecture — what the server actually is

### Strudel is a browser application

Strudel has **no built-in HTTP REST API**. The REPL lives in the browser; the
browser is the audio engine. Whatever serves the page is incidental.

The official repository has moved from GitHub to Codeberg:
- Active repo: https://codeberg.org/uzu/strudel
- GitHub (tidalcycles/strudel) is archived as of June 2025

### The bridge (`bridge/` in this repo)

Kate talks to Strudel through a small bridge of our own — `bridge/server.mjs` —
not through the Strudel monorepo. (An earlier revision of this doc described an
`@strudel/repl-control` package inside the monorepo; that package never existed
upstream. The protocol it described is real and unchanged — the implementation
just lives here instead.)

The bridge is one Node process on **localhost:8081** doing two jobs:

1. **Static server** — serves `bridge/public/index.html`, a page that runs the
   full Strudel REPL via `@strudel/web` (the official prebundled build,
   npm-installed and served from `node_modules` — no CDN, no build step).
2. **WebSocket hub** — the browser page connects as a `browser` client; Kate's
   Go harness connects as a `cli` client. Control messages relay cli → browser;
   state/error replies relay browser → cli, matched by `requestId`.

```bash
cd bridge
npm install     # once, online
npm start       # → http://localhost:8081, open it and click once
npm test        # relay smoke test (fake browser + fake cli)
```

### Offline-first

A hard project constraint: Kate must work offline or effectively offline.

- `npm install` is the **only** step that touches the network. After it, the
  page, the Strudel bundle, and the WebSocket hub are all served from disk.
- The page calls `initStrudel()` with **no prebake** — no sample packs are
  fetched at runtime. Strudel's built-in synths work with zero network.
- Sample packs (dirt-samples etc.) are normally fetched from GitHub at runtime;
  for Kate they must be vendored locally and registered with `samples()`
  pointing at local URLs. **Deferred — not set up yet.**

### Why not MIDI?

MIDI makes Kate a **controller** — she can drive note pitches and CC values on a
pre-authored Strudel template. The bridge makes her a **live coder** — she writes
and evaluates arbitrary Strudel code. Kate's identity is the latter.

| | MIDI | repl-control bridge |
|---|---|---|
| Kate writes Strudel patterns | No | Yes |
| Session state readable | No | Yes |
| Works offline | Yes | Yes (after one `npm install`) |
| Extra software required | Virtual MIDI driver | Node 18+ |

### Future: live audio input

The bridge page is also the planned home for microphone capture of live
artists: `getUserMedia` → WebAudio `AnalyserNode` → extracted features (onset,
pitch, RMS, tempo) → sent over the same WebSocket for the harness to fold into
the quantized queue. Raw waveform never reaches the model — only features. See
the extension-point comment at the bottom of `bridge/public/index.html`.

---

## Protocol reference

Source of truth: `bridge/server.mjs` (relay + error path),
`bridge/public/index.html` (action semantics), `internal/strudel/protocol.go`
(Go wire types). This section documents the same protocol.

**WebSocket URL:** `ws://localhost:8081` (binds 127.0.0.1 only; port override via
`BRIDGE_PORT` env var, mirrored by `strudel_url` in `kate.json`)

### Connection sequence

```
Kate (cli)  →  { "type": "handshake", "clientType": "cli" }
Browser     →  { "type": "handshake", "clientType": "browser" }
Browser     →  { "type": "state", "payload": { ...initialState } }
```

### Control message (cli → browser)

```json
{
  "type": "control",
  "action": "<action>",
  "payload": { ... },
  "requestId": "req_1"
}
```

Actions:

| Action | Required payload | Description |
|--------|-----------------|-------------|
| `evaluate` | `{ "code": "..." }` | Set editor code and evaluate |
| `play` | none | Re-evaluate current code |
| `stop` | none | Stop all playback |
| `toggle` | none | Toggle play/stop |
| `getState` | none | Request current state |

### State message (browser → cli)

```json
{
  "type": "state",
  "payload": {
    "code":      "...",
    "playing":   true,
    "error":     null,
    "timestamp": 1234567890
  },
  "requestId": "req_1"
}
```

A state message is sent after every command. `requestId` echoes back the sender's ID
for request/response matching. If an `evaluate` throws, the failure lands in
`payload.error` (the reply is still `type: "state"`); `type: "error"` is reserved
for bridge-level failures — no browser connected, unknown action.

### Error message

```json
{
  "type": "error",
  "error": "description",
  "requestId": "req_1"
}
```

---

## Channel model

The `evaluate` action **replaces the entire editor content**. There is no native
per-channel addressing. Kate manages named channels in Go by maintaining a
`map[channel]code` and rebuilding the combined Strudel code block on every change.

Each channel is a `$:` pattern block. Strudel runs all `$:` blocks concurrently:

```js
$: s("bd sd hh*4")          // channel: drums
$: note("c2 e2").s("bass")  // channel: bass
```

Operations:
- **`eval_pattern(ch, code)`** → update `channels[ch]`, rebuild, evaluate all
- **`stop_channel(ch)`** → delete `channels[ch]`, rebuild, evaluate remaining
- **`stop_all()`** → clear map, send `stop` action

`set_cps` is prepended to the combined code as a bare JS statement:
```js
setCps(0.5)
$: s("bd sd")
```

---

## Important: AudioContext user-gesture requirement

Browsers block audio until the user clicks the page. Before Kate can produce sound,
**someone must click once in the Strudel browser tab**. This is a hard browser
security constraint and cannot be bypassed. It only needs to happen once per session.

---

## Setup summary

```
1.  cd bridge && npm install        # once, the only online step
2.  npm start                       # bridge on http://localhost:8081
3.  Open http://localhost:8081 in a browser
4.  Click anywhere in the tab (AudioContext activation)
5.  Copy kate.example.json → kate.json, set provider/model/api_key
6.  go run ./cmd/kate
```
