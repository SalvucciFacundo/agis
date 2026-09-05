# Configuration

AGIS reads a single YAML file. The loader lives in `internal/config` and is fully tested (`internal/config/config_test.go`).

## File Format

```yaml
llm:
  provider: ollama      # llm provider: ollama | openai (or any OpenAI-compatible API)
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

tools:
  enabled: false                  # master switch: no tools without explicit opt-in
  web:
    enabled: false                # native web search and content extraction tools
    default_provider: duckduckgo  # duckduckgo | brave | tavily | searxng
    fetch_timeout: 15s            # HTTP request timeout
    max_fetch_bytes: 2097152      # 2MB response size limit
    user_agent: "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"
    providers:
      brave_api_key: ""           # Brave Search API key (masked in logs)
      tavily_api_key: ""          # Tavily Search API key (masked in logs)
      searxng_url: ""             # SearXNG instance URL (e.g. http://localhost:8080)
  docker:
    enabled: false
    image: alpine:3               # ephemeral container image
  ssh:
    enabled: false
    host: ""                      # e.g. vps.example
    user: ""                      # remote user
    key_path: ""                  # path to private key

gateway:
  enabled: false                  # master switch for external chat gateways
  telegram:
    enabled: false
    token: ""                     # bot token from @BotFather
    allowlist: []                 # permitted Telegram user IDs (fail-closed)
  discord:
    enabled: false
    token: ""                     # bot token from Discord Developer Portal
    allowlist: []                 # permitted Discord user IDs (fail-closed)

cron:
  enabled: false                  # master switch for background cron scheduler
  jobs:
    - name: "daily-health"
      schedule: "@every 1h"       # 5-field cron expression or duration interval
      prompt: "Check system health"
      session_id: "cron-health"   # optional: binds to persistent session ID
      target:
        adapter: "telegram"       # "telegram" | "discord"
        recipient: "123456789"    # target chat/channel ID

plugins:
  enabled: false                  # master switch for external plugins
  dir: ~/.agis/plugins            # plugin root directory (holds <name>/plugin.json)

mcp:
  enabled: false                  # master switch for Model Context Protocol (MCP) clients
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
      env:
        DEBUG: "mcp:*"
      disabled: false
    remote-tools:
      url: "http://localhost:8080/sse"
      disabled: false

webhook:
  enabled: false                  # master switch for webhook HTTP listener
  host: "127.0.0.1"               # binding address
  port: 8080                      # binding port
  path: "/webhook"                # endpoint path for HTTP POST events
  secret: ""                      # HMAC-SHA256 secret key for signature verification
  default_session_id: "webhook-events"
  target:
    adapter: "telegram"           # optional notification delivery target
    recipient: "123456789"

embeddings:
  enabled: false                  # master switch for dense vector embeddings and hybrid search
  provider: "ollama"              # "ollama" | "openai"
  model: "nomic-embed-text"       # default: "nomic-embed-text" (ollama) or "text-embedding-3-small" (openai)
  dimensions: 768                 # vector dimensions (0 for auto-detected)
  batch_size: 100                 # maximum items per embedding batch request (capped to 2048)

multimodal:
  enabled: false                  # master switch for vision and audio ingestion
  vision:
    enabled: false                # vision model multipart processing
    model: "gpt-4o"               # vision-capable model (e.g. gpt-4o, llama3.2-vision)
    max_image_size_mb: 10         # maximum image size limit in MB (default: 10)
  audio:
    enabled: false                # audio transcription processing
    provider: "openai"            # transcription provider ("openai" / Whisper)
    model: "whisper-1"            # transcription model
    max_audio_size_mb: 25         # maximum audio size limit in MB (default: 25)

subagents:
  enabled: true                   # master switch for native subagent delegation (delegate_task)
  max_concurrent: 3               # global concurrency limit for simultaneous subagents (clamped 1-10)
  max_depth: 1                    # maximum recursion depth limit (clamped 1-2)
  default_timeout: 60s            # execution timeout per subagent task (clamped 1s-300s)
  max_turns: 8                    # default maximum turns per subagent task (clamped 1-15)
```

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
| `tools.enabled` | `false` |
| `tools.docker.enabled` | `false` |
| `tools.docker.image` | `alpine:3` |
| `tools.ssh.enabled` | `false` |
| `tools.ssh.host` | (empty) |
| `tools.ssh.user` | (empty) |
| `tools.ssh.key_path` | (empty) |
| `gateway.enabled` | `false` |
| `gateway.telegram.enabled` | `false` |
| `gateway.telegram.token` | (empty) |
| `gateway.telegram.allowlist` | (empty, fail-closed) |
| `gateway.discord.enabled` | `false` |
| `gateway.discord.token` | (empty) |
| `gateway.discord.allowlist` | (empty, fail-closed) |
| `cron.enabled` | `false` |
| `cron.jobs` | `[]` |
| `plugins.enabled` | `false` |
| `plugins.dir` | `$AGIS_HOME/plugins` or `~/.agis/plugins` |
| `mcp.enabled` | `false` |
| `mcp.servers` | `{}` |
| `webhook.enabled` | `false` |
| `webhook.host` | `127.0.0.1` |
| `webhook.port` | `8080` |
| `webhook.path` | `/webhook` |
| `webhook.secret` | (empty) |
| `webhook.default_session_id` | `webhook-events` |
| `subagents.enabled` | `true` |
| `subagents.max_concurrent` | `3` |
| `subagents.max_depth` | `1` |
| `subagents.default_timeout` | `60s` |
| `subagents.max_turns` | `8` |

Defaults apply per-field: partial configuration retains safe defaults for omitted fields.

---

## Ecosystem Configuration Blocks

### 1. Gateway (`gateway`)
The `gateway` block configures chat platform adapters:
- `gateway.enabled`: Master switch. Must be `true` for `agis gateway` to run.
- `telegram.token` / `discord.token`: Platform bot API authentication tokens.
- `telegram.allowlist` / `discord.allowlist`: Static user ID lists. Messages from unlisted IDs are rejected and logged before any session allocation or LLM invocation.

### 2. Cron (`cron`)
The `cron` block configures scheduled background automations:
- `cron.enabled`: Master switch. Must be `true` for `agis cron run` daemon.
- `jobs`: List of scheduled job entries:
  - `name`: Unique alphanumeric identifier (e.g. `"daily-summary"`).
  - `schedule`: Standard 5-field cron expression (`"0 9 * * 1-5"`, `"*/30 * * * *"`, `@daily`, `@hourly`) or duration interval (`"@every 2h30m"`).
  - `prompt`: Autonomous instruction passed to `core.Brain.Step`.
  - `session_id` (optional): Bound session identifier (defaults to `cron:<name>`).
  - `target` (optional): Outbound notification destination containing `adapter` (`"telegram"` or `"discord"`) and `recipient` (chat/channel ID).

### 3. Plugins (`plugins`)
The `plugins` block configures external plugin discovery:
- `plugins.enabled`: Master switch for dynamic tool and skill registration.
- `plugins.dir`: Directory containing plugin subdirectories (each containing a `plugin.json` manifest).

### 4. Model Context Protocol (`mcp`)
The `mcp` block configures native MCP client servers (see [docs/mcp.md](mcp.md)):
- `mcp.enabled`: Master toggle for MCP tool discovery and execution.
- `mcp.servers`: Map of server configurations:
  - `command` / `args` / `env`: Executable binary, arguments, and environment variables for `stdio` subprocess transport.
  - `url`: HTTP SSE endpoint URL for `sse` network transport.
  - `disabled`: When `true`, skips initializing the server.

### 5. Webhook (`webhook`)
The `webhook` block configures the HTTP event listener:
- `webhook.enabled`: Master switch for `agis webhook run` listener.
- `host` & `port`: Network interface and port to bind (defaults to `127.0.0.1:8080`).
- `path`: HTTP endpoint path for POST events (defaults to `/webhook`).
- `secret`: Secret key used for HMAC-SHA256 signature verification via `X-Hub-Signature-256` or `X-Signature`.
- `default_session_id`: Default session prefix for incoming events (e.g. `webhook:<event_type>`).
- `target` (optional): Outbound chat destination to forward brain responses.

### 6. Subagents (`subagents`)
The `subagents` block configures isolated, ephemeral subagent task delegation (`delegate_task` tool):
- `subagents.enabled`: Master switch for subagent spawning and delegation (defaults to `true`).
- `subagents.max_concurrent`: Global limit on concurrently active child subagent loops (defaults to `3`, clamped `1-10`).
- `subagents.max_depth`: Maximum recursion depth allowed (defaults to `1`, hard maximum `2`).
- `subagents.default_timeout`: Execution deadline for a child subagent run (defaults to `60s`, clamped `1s-300s`).
- `subagents.max_turns`: Execution turn limit for child brain reasoning loops (defaults to `8`, clamped `1-15`).

---

## Security: 0600 Permissions

The config file may hold API tokens, SSH keys, or HMAC webhook secrets, so it is **expected to be mode `0600`**. If the file grants any permission to group or other, the loader emits a warning on stderr:

```
agis: warning: /home/you/.agis/config.yaml has permissions 0644; expected 0600
```

Fix file permissions with:
```bash
chmod 600 ~/.agis/config.yaml
```

---

## Practical Examples

### Telegram & Discord Chatbot Daemon

```yaml
llm:
  provider: openai
  model: gpt-4o
  api_key: sk-...

gateway:
  enabled: true
  telegram:
    enabled: true
    token: "123456789:ABCDefghIJKlmnoPQRstuvWXYZ"
    allowlist:
      - "987654321"
  discord:
    enabled: true
    token: "MTAwMjAwMzAwNDAwNTAwNjAw.xxxxxx.yyyyyy"
    allowlist:
      - "112233445566778899"
```

Start the gateway:
```bash
./bin/agis gateway run
```

### Scheduled Maintenance with Telegram Notifications

```yaml
cron:
  enabled: true
  jobs:
    - name: "morning-digest"
      schedule: "0 8 * * 1-5"
      prompt: "Summarize pending high-priority items and today's schedule"
      target:
        adapter: "telegram"
        recipient: "987654321"
```

Start the cron daemon:
```bash
./bin/agis cron run
```

List configured jobs:
```bash
./bin/agis cron list
```

### Webhook Event Listener with HMAC Authentication

```yaml
webhook:
  enabled: true
  host: "0.0.0.0"
  port: 8080
  path: "/events"
  secret: "super-secure-webhook-secret"
  target:
    adapter: "telegram"
    recipient: "987654321"
```

Start the webhook daemon:
```bash
./bin/agis webhook run --port 8080
```
