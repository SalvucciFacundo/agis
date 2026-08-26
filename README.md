# AGIS — Autonomous Go Intelligent System

A general-purpose autonomous agent in Go: a single static binary, SQLite persistence, zero external services at runtime. AGIS pairs the functional scope of [Hermes Agent](https://github.com/NousResearch/hermes-agent) (learning loop, multi-provider LLMs, multi-backend tools) with the hexagonal Go architecture of [GAIA](https://github.com/SalvucciFacundo/gaia) — built to run on anything from a $5 VPS to a laptop.

## Status

**M1 — Thinking agent with memory: DONE** (archived 2026-08-15). Verified 9/9 requirements, 11/11 scenarios, test suite green (50 test cases across 6 packages). M2–M5 are shipped and archived; M6 is designed in `spec.md` and tracked in [docs/roadmap.md](docs/roadmap.md).

## What works (M1)

- **Brain loop** — `Brain.Step` persists your message, loads the conversation tail, streams the provider's reply, and persists it. Tool calls are logged and ignored for now.
- **LLM provider port** — OpenAI and Ollama adapters over one shared OpenAI-compatible client. Provider and model come from config, no code changes to switch.
- **Memory** — SQLite (pure Go, no cgo) with FTS5 full-text search over messages and observations, embedded migrations, accent-insensitive search.
- **Minimal TUI** — Bubbletea viewport + input + spinner, streaming output, restores your latest conversation on startup.
- **Config** — `~/.agis/config.yaml` with safe defaults and a documented precedence order.

## Quickstart

Requirements: Go 1.26+, and [Ollama](https://ollama.com) running locally (or an OpenAI API key) for the model.

```bash
# build the binary
make build

# run (local Ollama, model llama3.2 by default)
./bin/agis

# or run directly without building
go run ./cmd/agis
```

The Makefile targets are `build`, `test`, `vet`, `lint`, `fmt`, `tidy`, `clean`. There is no `make run`; run `./bin/agis` or `go run ./cmd/agis`.

On first start AGIS creates `~/.agis/agis.db` and uses the defaults below. Type a message and press Enter; press `Ctrl+C` or `Esc` to quit.

## Configuration

Config lives in `~/.agis/config.yaml`. Defaults: provider `ollama`, model `llama3.2`, database at `~/.agis/agis.db`. See [docs/configuration.md](docs/configuration.md) for the full file format, the `-config` / `AGIS_HOME` / default precedence, and the 0600 permission requirement.

```yaml
llm:
  provider: ollama      # ollama | openai (anything else maps to the OpenAI-compatible client)
  model: llama3.2
  # api_key: ""          # required for openai, empty for local ollama
db:
  path: /home/you/.agis/agis.db
```

## Roadmap

| Milestone | Scope | State |
|---|---|---|
| M1 | Brain loop, LLM port, SQLite+FTS5 memory, minimal TUI, config | **DONE** |
| M2 | Learning loop: curator, nudges, session summarization, user model | **DONE** |
| M3 | Skills & persona: skill hub, SOUL.md, persona overlays | **DONE** |
| M4 | Tools, backends & permissions: Policy Guard, `agis policy`, `/permisos` | **DONE** |
| M5 | Full TUI: slash commands, session browse, interrupt-and-redirect | **DONE** |
| M6 | Gateway (Telegram/Discord first) + cron + ecosystem | planned |

Full detail in [docs/roadmap.md](docs/roadmap.md).

## Documentation

- [docs/architecture.md](docs/architecture.md) — hexagonal layout, ports, data flow, dependency direction
- [docs/memory.md](docs/memory.md) — SQLite schema, FTS5, embedded migrations, M2 learning-loop vision
- [docs/configuration.md](docs/configuration.md) — config file, precedence, defaults, security
- [docs/sessions.md](docs/sessions.md) — session lifecycle, 7 slash commands, snapshots (implemented in M5)
- [docs/permissions.md](docs/permissions.md) — permission system (implemented in M4)
- [docs/security.md](docs/security.md) — threat model and defenses (partially implemented)
- [docs/roadmap.md](docs/roadmap.md) — M1 shipped scope, M2–M6 plans, M1 review follow-ups

## Relationship to GAIA and Hermes

- **GAIA** — architectural DNA. Hexagonal layout, Bubbletea TUI, SQLite persistence, skill registry. AGIS is **not** a fork: it is a general-purpose agent with its own codebase, memory DB, and no coding-specific machinery.
- **Hermes** — functional reference. Learning loop, skills, multi-provider, multi-backend. AGIS reimplements that scope in Go at a fraction of the resource footprint.

## License

No license file has been added yet. Contact the maintainers before redistributing.
