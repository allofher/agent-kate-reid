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

Strudel has **no built-in HTTP REST API**. The REPL lives in the browser. The
"server" is a Vite dev server (or production static server) serving the browser app.

The official repository has moved from GitHub to Codeberg:
- Active repo: https://codeberg.org/uzu/strudel
- GitHub (tidalcycles/strudel) is archived as of June 2025

### Running Strudel locally

**Requirements:** Node.js, pnpm (not npm — the monorepo uses pnpm workspaces).

```bash
git clone https://codeberg.org/uzu/strudel
cd strudel
pnpm i
pnpm dev          # dev mode — repl-control WebSocket starts automatically on :8081
```

Production build for a more stable session:
```bash
pnpm build && pnpm preview
# Note: repl-control is tree-shaken out of production builds.
# For Kate, always use pnpm dev.
```

Docker alternative (production build only, no repl-control):
```bash
# https://github.com/LofiFren/strudel-docker
docker compose up -d   # → http://localhost:4321
```

---

## Communication: `@strudel/repl-control`

This is the chosen approach. A Vite plugin auto-starts a WebSocket server on port
**8081** when `pnpm dev` runs. The browser page connects as a `browser` client;
Kate's Go harness connects as a `cli` client. The server routes control messages
from cli → browser, and state messages from browser → cli.

### Why not MIDI?

MIDI makes Kate a **controller** — she can drive note pitches and CC values on a
pre-authored Strudel template. The bridge makes her a **live coder** — she writes
and evaluates arbitrary Strudel code. Kate's identity is the latter.

| | MIDI | repl-control |
|---|---|---|
| Kate writes Strudel patterns | No | Yes |
| Works with production builds | Yes | No (dev only) |
| Built into Strudel | Yes | Yes (dev mode) |
| Session state readable | No | Yes |
| Extra software required | Virtual MIDI driver | Just `pnpm dev` |

### Why not a custom WebSocket bridge?

More to build, nothing to gain for a local band setup.
`repl-control` is already wired into the dev server. No bridge server needed.

---

## Protocol reference

Source: `packages/repl-control/` in the Strudel monorepo.

**WebSocket URL:** `ws://localhost:8081` (localhost only, hardcoded in server.mjs)

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
for request/response matching.

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
1.  git clone https://codeberg.org/uzu/strudel && cd strudel
2.  pnpm i && pnpm dev
3.  Open http://localhost:3000 in browser
4.  Click anywhere in the browser tab (AudioContext activation)
5.  Copy kate.example.json → kate.json, set provider/model/api_key
6.  go run ./cmd/kate
```
