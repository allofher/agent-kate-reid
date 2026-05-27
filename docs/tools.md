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

This section is critical before writing any tool implementation.

### Strudel is a browser application

Strudel has **no built-in HTTP REST API or standalone server process**. The REPL lives
entirely in the browser. When the docs say "run the server," they mean a static asset
server (Vite dev server or `pnpm preview`) serving the browser app. There is no socket
Kate can POST code to out of the box.

The official repository has moved from GitHub to Codeberg:
- New home: https://codeberg.org/uzu/strudel
- GitHub (tidalcycles/strudel) is archived as of June 2025

### Running Strudel locally

**Requirements:** Node.js, pnpm (not npm — the monorepo uses pnpm workspaces).

```bash
# Clone from the active repo
git clone https://codeberg.org/uzu/strudel
cd strudel

# Install (pnpm required — npm will not work)
pnpm i

# Development mode (hot reload, enables repl-control WebSocket)
pnpm dev
# → REPL available at http://localhost:3000 (default Vite port)

# Production build + preview
pnpm build
pnpm preview
```

Docker alternative (wraps the production build):
```bash
# From https://github.com/LofiFren/strudel-docker
docker compose up -d
# → REPL at http://localhost:4321
```

The Docker image is ~3 GB (includes Node runtime + build toolchain). First run clones and
builds; subsequent runs start instantly.

---

### How to talk to the REPL from outside the browser

Three approaches exist in the wild. Each has different trade-offs for Kate.

#### Option A — `@strudel/repl-control` (dev-mode WebSocket)

Strudel ships a built-in `@strudel/repl-control` package that opens a WebSocket server
when running `pnpm dev`. CLI scripts connect to it to send code.

**Constraint:** This is explicitly excluded from production builds. It only works with
`pnpm dev`, not `pnpm preview` or the Docker image.

**Verdict for Kate:** Fine for local development/testing. Not viable for a stable band
setup where others may run a production build.

#### Option B — Express + WebSocket bridge (recommended for Kate)

Run a companion Node.js server alongside Strudel. The bridge exposes HTTP and/or
WebSocket endpoints. The Strudel page loads a small client script that connects back
to the bridge and drives the CodeMirror editor when it receives messages.

Precedent: the [param-strudels](https://github.com/Paramstr/param-strudels) project
uses exactly this pattern — an Express server (`pnpm api`) translates requests into
CodeMirror editor actions in the browser.

**Architecture:**

```
Kate (Go) ──HTTP/WS──▶ bridge server (Node) ──WS──▶ Strudel browser page
                                                          │
                                                    CodeMirror
                                                    editor DOM
```

Kate's Go `strudel.Client` points at the bridge server, not at the Strudel REPL directly.
The bridge owns the browser session.

**Verdict for Kate:** Best option. Decouples Kate from the browser, works with any
Strudel build (dev or production), and gives the band a single stable endpoint.

#### Option C — Playwright browser automation

Automate the browser directly using Playwright. Access CodeMirror internals via
`editor.__view.dispatch(...)`. No bridge server needed.

Precedent: [strudel-mcp-server](https://github.com/williamzujkowski/strudel-mcp-server)
and [strudel-server](https://github.com/micahkepe/strudel-server) both use this
approach. The MCP server reports ~80% faster execution than keyboard simulation.

**Verdict for Kate:** High fidelity but heavy dependency (headless Chromium). Good for
single-machine setups, awkward when the band runs Strudel on separate hardware.

---

### Recommended setup for Kate

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Band machine(s)                           │
│                                                                     │
│   Browser (Strudel REPL)  ◀──WS──  bridge server  ◀──HTTP/WS──  Kate│
│         ↕ audio                    (Node, port N)   (Go harness)   │
│   Human performers                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

1. The band runs Strudel in a browser (any build).
2. The bridge server runs alongside it (or on the same machine) and injects a
   small listener script into the Strudel page.
3. Kate's Go harness hits the bridge via the URL set in `kate.json` (`strudel_url`).
4. The bridge translates Kate's tool calls into CodeMirror editor actions.

**What needs to be built:**
- A minimal bridge server (Node/Express + WebSocket) — can live in this repo under
  `bridge/` as a companion service, or be extracted as a separate project.
- The injected browser client script that connects to the bridge and drives CodeMirror.
- Kate's Go tool implementations pointing at the bridge HTTP endpoints.

---

## Open questions before implementation

1. **Bridge message protocol**: define the JSON schema for `eval`, `stop`, `get_state`,
   `set_cps` messages between Kate's Go tools and the bridge server.
2. **Channel model**: Strudel doesn't have named channels natively. We likely represent
   them as separate `$: ` mini-notation blocks or separate `hush`-able pattern variables.
   Need to validate which approach survives eval/hush correctly.
3. **Session state shape**: what does `get_session_state()` actually return? The bridge
   needs to read current editor content and return it parsed by channel/pattern.
4. **Multi-performer state**: if other performers are on separate machines, does the
   bridge aggregate their state, or does each machine run its own bridge with a shared
   state store?
