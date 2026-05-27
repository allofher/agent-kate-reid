# agent-kate-reid

Kate Reid is an AI musician agent that plays alongside human performers using the [Strudel](https://strudel.cc/) live-coding REPL framework.

She listens to what the band is doing, thinks with a language model, and uses a set of Strudel-specific tools to write and execute patterns on a local Strudel server — in real time, as part of a live session.

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Band / session                    │
│  human performers  ←→  local Strudel server          │
│                              ↑                       │
│                         Strudel tools                │
│                              ↑                       │
│                        agent-kate-reid               │
│                              ↑                       │
│                      inference backend               │
│              (Anthropic / OpenAI / OpenRouter /      │
│               Ollama / …)                            │
└──────────────────────────────────────────────────────┘
```

The harness runs locally. It connects Kate's inference backend to the local Strudel server via a set of purpose-built tools. Kate never uses generic shell or Python execution; every action she can take maps directly to something meaningful in Strudel.

---

## Strudel tools

Kate's tool set is designed around the Strudel REPL API:

| Tool | What it does |
|------|-------------|
| `eval_pattern` | Send a Strudel pattern string to the REPL for immediate playback |
| `stop_pattern` | Silence a specific pattern channel |
| `stop_all` | Silence all active patterns |
| `get_state` | Read back the currently playing patterns and their last-evaluated code |
| `set_tempo` | Change the global BPM |
| `list_samples` | Enumerate available sample banks |

More tools will be added as Kate's musical vocabulary grows.

---

## Inference providers

Set `PROVIDER` in your `.env` to select a backend:

| Value | Description |
|-------|-------------|
| `anthropic` | Anthropic API (Claude models) |
| `openai` | OpenAI API |
| `openrouter` | OpenRouter (proxied access to many models) |
| `ollama` | Local Ollama instance |

See `.env.example` for all configuration keys.

---

## Getting started

### Prerequisites

- Go 1.23+
- A running local [Strudel](https://strudel.cc/) server (or the desktop app with its API enabled)
- API credentials for your chosen inference provider

### Run

```bash
cp .env.example .env
# fill in .env

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
internal/
  agent/            Kate: wires inference + tools
  strudel/          HTTP client for the Strudel server
  tools/            Strudel-specific tool definitions
.env.example        configuration template
```

---

## Contributing

Kate is a work in progress. If you want to extend her musical abilities, the right place is `internal/tools/` — add a new tool there, register it in the `Registry`, and expose it to the inference provider.
