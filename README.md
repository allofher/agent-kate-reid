# agent-kate-reid

Kate Reid is an AI musician agent that plays alongside human performers using the [Strudel](https://strudel.cc/) live-coding REPL framework.

She listens to what the band is doing, thinks with a language model, and uses a set of Strudel-specific tools to write and execute patterns on a local Strudel server — in real time, as part of a live session.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Band / session                    │
│  human performers  ←→  browser tab (Strudel REPL)    │
│                              ↑                       │
│                     bridge/ (ws hub, :8081)          │
│                              ↑                       │
│                         Strudel tools                │
│                              ↑                       │
│                        agent-kate-reid               │
│                              ↑                       │
│                      inference backend               │
│              (Anthropic / OpenAI / Ollama)           │
└──────────────────────────────────────────────────────┘
```

The harness runs locally. It connects Kate's inference backend to a Strudel REPL running in a local browser tab, via a small WebSocket bridge (`bridge/`) and a set of purpose-built tools. Kate never uses generic shell or Python execution; every action she can take maps directly to something meaningful in Strudel.

Everything runs offline: the bridge serves a vendored Strudel bundle from disk, and the only step that ever touches the network is the one-time `npm install` (plus your inference provider, unless you run a local model via Ollama).

---

## Strudel tools

Kate's tool set is designed around the Strudel REPL API:

| Tool | What it does |
|------|-------------|
| `eval_pattern` | Queue a Strudel pattern for one of Kate's named channels |
| `stop_channel` | Queue silencing of a specific pattern channel |
| `stop_all` | Queue silencing of all of Kate's patterns |
| `get_session_state` | Read back the currently playing code immediately (Kate's and the other players') |
| `set_cps` | Queue a global tempo change (cycles per second) |

### The quantized queue

Mutating tools don't hit the Strudel server directly. They enqueue operations on the
harness queue, and a scheduler applies the queue at deterministic intervals — either
**cycle-aligned** (every N cycles, derived from the current cps) or **wall-clock**
(every N seconds), configurable under `harness` in `kate.json`. This way Kate's
additions and layerings land on musical boundaries, and the state she reads is never
mid-flight. Tool calls return `"queued"` immediately so she can stack several layers
in one window; after each tick she receives a report of what was applied (including
any eval errors) plus the fresh session state, and takes her next turn.

More tools will be added as Kate's musical vocabulary grows.

---

## Inference providers

Set `provider` in `kate.json` to select a backend:

| Value | Description |
|-------|-------------|
| `anthropic` | Anthropic API (Claude models) |
| `openai` | OpenAI API |
| `ollama` | Local Ollama instance (fully offline) |

See `kate.example.json` for all configuration keys.

---

## Getting started

### Prerequisites

- Go 1.23+
- Node.js 18+ (for the bridge)
- API credentials for your chosen inference provider (or a local Ollama)

### Run

```bash
# 1. Start the bridge (first time: npm install — the only online step)
cd bridge && npm install && npm start

# 2. Open http://localhost:8081 in a browser and click once
#    (browser autoplay policy: no click, no sound)

# 3. Configure and run Kate
cp kate.example.json kate.json
# fill in provider/model/api_key
go run ./cmd/kate
```

### Build

```bash
go build -o kate ./cmd/kate
./kate
```

---

## Project layout

```
cmd/kate/           entry point
bridge/             WebSocket hub + offline Strudel page (Node, port 8081)
internal/
  agent/            Kate: wires inference + tools + harness
  config/           kate.json loading and validation
  harness/          quantized queue + tick scheduler
  provider/         inference backends (Anthropic, OpenAI, Ollama)
  strudel/          WebSocket client for the bridge
  tools/            Strudel-specific tool definitions
docs/tools.md       tool design + bridge architecture + wire protocol
kate.example.json   configuration template
```

---

## Contributing

Kate is a work in progress. If you want to extend her musical abilities, the right place is `internal/tools/` — add a new tool there, register it in the `Registry`, and expose it to the inference provider.
