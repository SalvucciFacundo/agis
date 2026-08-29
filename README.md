# AGIS — Autonomous Go Intelligent System

<p align="center">
  <img src="assets/hero_banner.png" alt="AGIS — Autonomous Go Intelligent System" width="100%">
</p>

<p align="center">
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://github.com/SalvucciFacundo/agis/releases"><img src="https://img.shields.io/github/v/release/SalvucciFacundo/agis?logo=github" alt="Release"></a>
  <a href="https://github.com/SalvucciFacundo/agis/actions/workflows/ci.yml"><img src="https://github.com/SalvucciFacundo/agis/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="docs/roadmap.md"><img src="https://img.shields.io/badge/Milestones-M1--M6%20Shipped-brightgreen" alt="Milestones M1-M6 Shipped"></a>
  <a href="https://github.com/SalvucciFacundo/agis"><img src="https://img.shields.io/badge/PRs-welcome-brightgreen" alt="PRs Welcome"></a>
</p>

A general-purpose autonomous agent in Go: a single static binary, SQLite persistence, zero external services at runtime. AGIS pairs the functional scope of [Hermes Agent](https://github.com/NousResearch/hermes-agent) (learning loop, multi-provider LLMs, multi-backend tools, ecosystem integrations) with the hexagonal Go architecture of [GAIA](https://github.com/SalvucciFacundo/gaia) — built to run on anything from a $5 VPS to a laptop.

## Status

**Milestones M1–M6: ALL SHIPPED & VERIFIED ✅**
- **M1 (Skeleton & Memory)**: Hexagonal core, LLM port (Ollama/OpenAI), SQLite+FTS5 memory, Bubbletea TUI.
- **M2 (Learning Loop)**: Memory curator, nudges, close-time session summarization, user model aggregation.
- **M3 (Skills & Persona)**: Skill Hub, `SOUL.md` durable identity, personality presets, guided evolution.
- **M4 (Tools & Policy)**: Tool-calling loop, Local/Docker/SSH backends, Policy Guard, `/permisos` panel.
- **M5 (Full TUI & Sessions)**: Session Manager, slash commands (`/new`, `/save`, `/list`, `/restore`, `/rename`, `/compress`, `/snapshot`).
- **M6 (Ecosystem & Integrations)**: Telegram/Discord Gateway, Cron Scheduler, Plugin Manager, HMAC Webhooks, CLI daemons.

Full milestone history in [docs/roadmap.md](docs/roadmap.md).

## Core Capabilities

- **Brain Loop** — `Brain.Step` persists turns, loads history, injects memory & skills, streams model tokens, evaluates tool calls, and persists final assistant turns.
- **Multi-Provider LLM** — Ollama, OpenAI, and any OpenAI-compatible API over a unified client with streaming SSE.
- **SQLite + FTS5 Memory** — Pure Go SQLite with full-text search over conversations, messages, observations, and snapshots.
- **Learning & Memory Loop** — Continuous observation extraction (Curator), session summarization, and user model confidence synthesis.
- **Skill Hub & Persona** — Agentskills.io-compatible Markdown skill loading, runtime skill creation, durable `SOUL.md`, and dynamic personality overlays.
- **Policy Guard & Tool Backends** — Multi-tier security postures (`sandbox`, `standard`, `full`), fail-closed approval, audit logging, and Local/Docker/SSH tool backends.
- **Chat Gateway Multiplexer** — Concurrent Telegram and Discord bot adapters, user allowlists, session multiplexing, and message chunking.
- **Cron Scheduler Daemon** — Autonomous scheduled tasks with 5-field cron and `@every` duration expressions, sandbox policy, and chat notification delivery.
- **Plugin Manager** — Dynamic external plugin discovery (`plugin.json`), tool runner bridge, skill extraction, and persistent state management.
- **HMAC Webhook Server** — HTTP event listener with constant-time HMAC-SHA256 signature verification and brain event dispatching.

## 🚀 Installation

AGIS is distributed as a single static binary with zero external runtime dependencies. Choose your preferred installation method:

### 1. Automatic One-Line Installer (Recommended)

**Linux & macOS (Bash/Zsh):**
```bash
curl -fsSL https://raw.githubusercontent.com/SalvucciFacundo/agis/main/install.sh | bash
```
*Auto-detects Debian, Ubuntu, Arch, Fedora, Alpine, CentOS, openSUSE, and macOS (Intel & Apple Silicon).*

**Windows (PowerShell 5.1 / 7+):**
```powershell
iwr -useb https://raw.githubusercontent.com/SalvucciFacundo/agis/main/install.ps1 | iex
```

---

### 2. Go Install (Cross-Platform)

If you have Go installed on your machine:
```bash
go install github.com/SalvucciFacundo/agis/cmd/agis@latest
```

---

### 3. Prebuilt Packages & Binaries (GitHub Releases)

Download precompiled standalone binaries, `.deb`, `.rpm`, `.apk`, `.tar.gz`, or `.zip` archives directly from [GitHub Releases](https://github.com/SalvucciFacundo/agis/releases):

- **Debian / Ubuntu / Mint / Pop!_OS:** `sudo dpkg -i agis_*_linux_amd64.deb`
- **Fedora / RHEL / CentOS / Rocky:** `sudo rpm -ivh agis_*_linux_amd64.rpm`
- **Alpine Linux:** `sudo apk add --allow-untrusted agis_*_linux_amd64.apk`
- **Arch Linux:** Standalone binary or install via script
- **macOS Universal Binary:** Standalone `tar.gz` for both Apple Silicon (M1/M2/M3/M4) and Intel
- **Windows Executable:** `agis_*_windows_amd64.zip`

---

### 4. Build from Source

Requirements: Go 1.26+ (or 1.24+).

```bash
# Clone the repository
git clone https://github.com/SalvucciFacundo/agis.git
cd agis

# Build static binary
make build

# Launch interactive TUI (defaults to local Ollama with llama3.2)
./bin/agis

# Or run directly without building
go run ./cmd/agis
```

The `Makefile` targets are: `build`, `test`, `vet`, `lint`, `fmt`, `tidy`, `clean`.

## CLI Subcommands

AGIS provides modular daemons and management subcommands alongside the default interactive TUI:

```bash
# 1. Chat Gateway (Telegram & Discord daemon)
./bin/agis gateway [run] [--config config.yaml]

# 2. Cron Scheduler Daemon
./bin/agis cron run [--config config.yaml]
./bin/agis cron list [--config config.yaml]

# 3. External Plugins Management
./bin/agis plugins list [--dir ~/.agis/plugins]
./bin/agis plugins enable <plugin_name>
./bin/agis plugins disable <plugin_name>
./bin/agis plugins inspect <plugin_name>

# 4. Webhook HTTP Listener
./bin/agis webhook run [--port 8080] [--host 127.0.0.1] [--path /webhook]

# 5. Policy Guard CLI
./bin/agis policy show
./bin/agis policy set <rule>
./bin/agis policy rm <rule>
./bin/agis policy tier <sandbox|standard>
```

## Configuration

Config lives in `~/.agis/config.yaml` (or `$AGIS_HOME/config.yaml`). See [docs/configuration.md](docs/configuration.md) for the full specification, defaults, and security guidance.

```yaml
llm:
  provider: ollama      # ollama | openai
  model: llama3.2
  # api_key: ""         # required for openai, empty for local ollama

db:
  path: /home/you/.agis/agis.db

gateway:
  enabled: false
  telegram:
    enabled: false
    token: ""
    allowlist: []
  discord:
    enabled: false
    token: ""
    allowlist: []

cron:
  enabled: false
  jobs:
    - name: "daily-health"
      schedule: "@every 1h"
      prompt: "Check system health"
      target:
        adapter: "telegram"
        recipient: "123456"

plugins:
  enabled: false
  dir: "~/.agis/plugins"

webhook:
  enabled: false
  port: 8080
  path: "/webhook"
  secret: ""
```

## Roadmap

| Milestone | Scope | State |
|---|---|---|
| M1 | Brain loop, LLM port, SQLite+FTS5 memory, minimal TUI, config | **DONE** |
| M2 | Learning loop: curator, nudges, session summarization, user model | **DONE** |
| M3 | Skills & persona: skill hub, SOUL.md, persona overlays | **DONE** |
| M4 | Tools, backends & permissions: Policy Guard, `agis policy`, `/permisos` | **DONE** |
| M5 | Full TUI: slash commands, session browse, interrupt-and-redirect | **DONE** |
| M6 | Gateway (Telegram/Discord) + cron + plugins + webhooks | **DONE** |

Full detail in [docs/roadmap.md](docs/roadmap.md).

## Documentation

- [docs/architecture.md](docs/architecture.md) — Hexagonal layout, ports, data flow, ecosystem architecture
- [docs/configuration.md](docs/configuration.md) — Configuration file, precedence, defaults, security, examples
- [docs/sessions.md](docs/sessions.md) — Session lifecycle, slash commands, snapshots
- [docs/permissions.md](docs/permissions.md) — Policy Guard, permission system, audit logging
- [docs/security.md](docs/security.md) — Threat model, sandbox posture, HMAC verification, allowlists
- [docs/roadmap.md](docs/roadmap.md) — Milestone history, shipped scopes, verification details

## Relationship to GAIA and Hermes

- **GAIA** — architectural DNA. Hexagonal layout, Bubbletea TUI, SQLite persistence, skill registry. AGIS is **not** a fork: it is a general-purpose agent with its own codebase, memory DB, and no coding-specific machinery.
- **Hermes** — functional reference. Learning loop, skills, multi-provider, multi-backend, ecosystem integrations. AGIS reimplements that scope in Go at a fraction of the resource footprint.

## Contributing

Contributions, issues, and feature requests are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

This project is licensed under the [MIT License](LICENSE) - see the [LICENSE](LICENSE) file for details.
