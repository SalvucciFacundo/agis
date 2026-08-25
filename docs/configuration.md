# Configuration

AGIS reads a single YAML file. The loader lives in `internal/config` and is fully tested (`internal/config/config_test.go`).

## File format

```yaml
llm:
  provider: ollama      # llm provider: ollama | openai
  model: llama3.2       # model name sent to the provider
  api_key: ""           # API key (openai); empty is valid for local ollama
db:
  path: /home/you/.agis/agis.db   # SQLite database file
memory:
  learning_enabled: true          # master switch for the learning loop
  recall_limit: 10                # top-N observations injected per turn
  nudge_every: 10                 # curator runs every N assistant messages
  close_timeout: 30s              # bounded wait for session close on quit
agent:
  personalities: {}               # custom /personality presets (name -> text)
  evolution_enabled: true         # derived persona layer from user model
skills:
  enabled: true                   # skill hub: loading, matching, creation
  dir: ~/.agis/skills             # where skill files live
```

The `llm` and `db` blocks are the M1 core. The `memory` block tunes the learning loop (curator, summarizer, user model, recall).

`provider` selects the adapter at startup: `ollama` picks the local Ollama adapter, and any other value (including `openai`) picks the OpenAI-compatible client (`internal/adapters/llm/provider.go:14`).

`api_key` is intentionally untouched by defaulting: an empty key is a valid value for local backends (`internal/config/config.go:101`).

## Precedence

Resolution order, highest first:

1. **`-config` flag** — `./bin/agis -config /path/to/config.yaml`
2. **`AGIS_HOME`** — the environment variable; the config file is `$AGIS_HOME/config.yaml`
3. **Default path** — `~/.agis/config.yaml`

The same precedence governs the database path default (`$AGIS_HOME/agis.db` or `~/.agis/agis.db`) via `agisDir()` (`internal/config/config.go:124`).

A missing file is **not an error**: the loader falls back to built-in defaults. A present-but-invalid file is an error and aborts startup.

## Defaults

| Field | Default |
|---|---|
| `llm.provider` | `ollama` |
| `llm.model` | `llama3.2` |
| `db.path` | `$AGIS_HOME/agis.db` or `~/.agis/agis.db` |
| `memory.learning_enabled` | `true` |
| `memory.recall_limit` | `10` |
| `memory.nudge_every` | `10` |
| `memory.close_timeout` | `30s` |
| `agent.evolution_enabled` | `true` |
| `agent.personalities` | (empty) |
| `skills.enabled` | `true` |
| `skills.dir` | `$AGIS_HOME/skills` or `~/.agis/skills` |

Defaults apply per-field: a config file that sets only `llm.model` keeps the default provider and database path (`applyDefaults`, `internal/config/config.go:102`).

## Memory block

The learning loop runs by default. When `learning_enabled: false`, the brain is built without curator and summarizer: no recall injection, no periodic curation, no close-time summarization (`cmd/agis/main.go`). The TUI still closes sessions, but with no closer wired it is a fast no-op.

| Field | Semantics |
|---|---|
| `learning_enabled` | Master switch for the whole loop |
| `recall_limit` | Top-N observations prepended as a system message each turn; zero or negative restores the default |
| `nudge_every` | Curator fires every N assistant messages; an explicit `0` disables nudging and survives defaulting on purpose |
| `close_timeout` | How long quitting waits for the summarizer before giving up (Go duration string, e.g. `30s`, `1m`) |

Several values are intentionally exempt from defaulting because their zero/false forms are meaningful — `learning_enabled: false`, `nudge_every: 0`, `agent.evolution_enabled: false`, and `skills.enabled: false`. Keys absent from the file always keep their defaults, so a partial block such as `memory: {recall_limit: 5}` leaves every other default intact.

On quit (CtrlC/Esc) the TUI shows a `closing...` status and waits up to `close_timeout` for the session summary to finish before exiting. While streaming, the first press cancels the stream and commits the partial reply; the second force-quits without closing.

## Security: 0600

The config file may hold an API key, so it is **expected to be mode `0600`**. If the file grants any permission to group or other, the loader emits a warning on stderr (or the `WithWarnWriter` writer in tests):

```
agis: warning: /home/you/.agis/config.yaml has permissions 0644; expected 0600
```

The check is advisory in M1 — it warns, it does not refuse to start. See [docs/security.md](docs/security.md) for the security context. Fix a loose file with:

```bash
chmod 600 ~/.agis/config.yaml
```

## Practical examples

Switch to OpenAI:

```yaml
llm:
  provider: openai
  model: gpt-4o-mini
  api_key: sk-...   # then: chmod 600 ~/.agis/config.yaml
```

Point at any other OpenAI-compatible endpoint by treating it as a stand-in for the OpenAI adapter (the provider value `openai` is a catch-all, `internal/adapters/llm/provider.go:14`):

```bash
AGIS_HOME=/srv/agis ./bin/agis -config /srv/agis/config.yaml
```

`AGIS_HOME` also relocates the database — useful for a portable or multi-instance setup without touching the file.

## Not yet implemented

The `-config` flag is the only CLI flag in M1; `agis config set/get` and `agis model` are designed in `spec.md` but not yet implemented.
