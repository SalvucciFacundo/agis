# M6 — Ecosystem (Delta Spec)

Delta specification for `m6-ecosystem`: Gateway (Telegram + Discord), Cron Scheduler, Plugin Manager, Webhooks, Config Loader extensions, and CLI subcommands.

---

## gateway (NEW)

### AGIS-M6-GTW-001: Gateway Multiplexer and Adapter Port
The system MUST provide a Gateway Multiplexer (`internal/gateway/`) that manages multiple chat platform adapters concurrently. Each adapter MUST implement the `Adapter` interface:
- `Name() string`: Returns the unique adapter identifier (e.g. `"telegram"`, `"discord"`).
- `Start(ctx context.Context) error`: Connects to the upstream platform and begins listening for incoming messages.
- `Stop() error`: Gracefully shuts down connection listeners and completes inflight message processing.
- `Send(ctx context.Context, target string, msg string) error`: Sends an outbound message to a specified recipient/channel ID.

The Gateway Multiplexer MUST start all enabled adapters when initialized and coordinate graceful shutdown across all adapters when context is canceled or `Stop()` is called.

#### Scenario: Multiplexer starts all enabled adapters
- GIVEN a gateway configuration with Telegram enabled and Discord enabled
- WHEN the Gateway Multiplexer `Start(ctx)` is invoked
- THEN both Telegram and Discord adapters initialize their listeners concurrently and log their active status

#### Scenario: Graceful shutdown cancels all adapter listeners
- GIVEN running Telegram and Discord adapters managed by the multiplexer
- WHEN the multiplexer `Stop()` is invoked
- THEN both adapters disconnect gracefully without dropping inflight send operations

---

### AGIS-M6-GTW-002: Telegram Adapter
The Telegram adapter MUST connect to the Telegram Bot API using the configured bot token. It MUST poll for updates or receive webhook updates, translate incoming Telegram messages into internal Gateway events, and deliver outbound replies via the Telegram Bot API `sendMessage` endpoint. Messages exceeding Telegram's 4096-character limit MUST be chunked or split cleanly without dropping content.

#### Scenario: Inbound Telegram message received
- GIVEN a configured and running Telegram adapter
- WHEN an authorized Telegram user sends `"Hello AGIS"`
- THEN the adapter transforms the payload into a standard Gateway message event with user ID, chat ID, and text content

#### Scenario: Long message split before sending
- GIVEN a brain response containing 5000 characters
- WHEN the Telegram adapter delivers the response via `Send`
- THEN the message is chunked into parts each under 4096 characters and delivered in sequence

---

### AGIS-M6-GTW-003: Discord Adapter
The Discord adapter MUST connect to the Discord Gateway via WebSocket or REST API using the configured bot token. It MUST listen for message creation events in permitted channels or direct messages, map Discord user IDs and channel IDs to internal Gateway events, and deliver outbound replies via Discord REST API. Messages exceeding Discord's 2000-character limit MUST be split cleanly into multiple sequential messages.

#### Scenario: Inbound Discord message received
- GIVEN a configured and running Discord adapter
- WHEN an authorized Discord user sends a message in a watched channel
- THEN the adapter converts the Discord event into an internal Gateway event with author ID, channel ID, and text content

#### Scenario: Outbound message chunking on Discord
- GIVEN a brain response containing 3000 characters
- WHEN the Discord adapter sends the message to a channel
- THEN the message is split into chunks of at most 2000 characters and sent sequentially

---

### AGIS-M6-GTW-004: User Allowlist Security Enforcement
Each gateway adapter MUST enforce a static user ID allowlist configured in `config.yaml`. Incoming messages from user IDs not present in the adapter's allowlist MUST be dropped or rejected immediately before allocating sessions or invoking the brain. Dropped unauthorized interactions MUST be logged with sender ID and platform name.

#### Scenario: Authorized user message accepted
- GIVEN a Telegram adapter with allowlist `["123456789"]`
- WHEN user `"123456789"` sends `"ping"`
- THEN the message passes allowlist validation and reaches the session router

#### Scenario: Unauthorized user message rejected
- GIVEN a Discord adapter with allowlist `["987654321"]`
- WHEN user `"111222333"` sends a message
- THEN the message is dropped, no brain invocation occurs, and an unauthorized access warning is logged

---

### AGIS-M6-GTW-005: Sandbox Policy Guard & Non-Interactive Auto-Deny
Gateway execution MUST run under the `sandbox` security posture without interactive human-in-the-loop prompts. When a tool call evaluated by `PolicyGuard` yields `DecisionAsk` (or requires interactive privilege escalation), the gateway brain runner MUST automatically deny the execution (`"blocked by policy"`), preventing remote deadlock. Persistent `always` allow rules configured in `policy.yaml` MUST continue to execute normally.

#### Scenario: Unapproved tool call auto-denied in gateway session
- GIVEN a gateway-routed turn triggering a tool call that requires approval (`DecisionAsk`)
- WHEN evaluated in the non-interactive gateway environment
- THEN the approver auto-denies the action, logs the denied attempt to the policy audit log, and informs the model the action was blocked

#### Scenario: Persistent allow rule executes in gateway session
- GIVEN a tool command with an explicit `always` allow rule in `policy.yaml`
- WHEN evaluated in a gateway session
- THEN the guard returns `DecisionAllow` and the tool executes successfully

---

### AGIS-M6-GTW-006: Session Routing and Brain Execution
The Gateway MUST maintain deterministic session mapping using `session.SessionManager`. Each platform user/channel combination MUST map to a unique session key (e.g. `gateway:telegram:<chat_id>` or `gateway:discord:<channel_id>`). Inbound messages MUST be routed to `core.Brain.Step`, streaming text chunks and aggregating the final response to send back via the originating adapter.

#### Scenario: Session continuity across consecutive messages
- GIVEN an existing gateway session for Telegram chat ID `445566`
- WHEN the user sends a second message within the same chat
- THEN the brain receives the previous conversation history from `SessionManager` and appends the new turn

---

## cron (NEW)

### AGIS-M6-CRN-001: Cron Scheduler Engine
The system MUST provide a Cron Scheduler (`internal/cron/`) capable of executing periodic background tasks defined in `config.yaml`.
- The scheduler MUST support standard 5-field cron expressions (e.g. `"0 9 * * *"` or `"*/15 * * * *"`) and duration intervals (e.g. `"@every 1h"`).
- Invalid cron expressions MUST be rejected at config validation time before the scheduler starts.
- The scheduler MUST run in a background goroutine and shut down cleanly when its parent context is canceled.

#### Scenario: Scheduler parses and registers valid cron jobs
- GIVEN a config with two jobs: `"@every 30m"` and `"0 8 * * 1-5"`
- WHEN the cron engine initializes
- THEN both jobs are scheduled with their calculated next run timestamps

#### Scenario: Invalid cron expression fails validation
- GIVEN a job configured with schedule `"invalid-cron-format"`
- WHEN the cron scheduler attempts initialization
- THEN initialization returns a descriptive parsing error and the scheduler does not start

---

### AGIS-M6-CRN-002: Job Execution via Brain
When a cron job triggers, the scheduler MUST execute the job's configured prompt through `core.Brain.Step`:
- If `session_id` is configured, the job MUST bind to that session; otherwise, it MUST execute in an isolated ephemeral session named `cron:<job_name>`.
- The execution MUST run non-interactively under the `sandbox` policy guard with auto-deny on unapproved tool actions.
- Job execution outcomes (start time, duration, status, error if any) MUST be logged.

#### Scenario: Cron job executes prompt via Brain
- GIVEN a configured cron job named `"daily-summary"` with prompt `"Summarize pending tasks"`
- WHEN the job trigger time arrives
- THEN the scheduler invokes `Brain.Step` with the prompt, loads repository context, and produces a summary output

---

### AGIS-M6-CRN-003: Gateway Notification Delivery
A cron job MAY define an optional `target` block containing `adapter` (e.g. `"telegram"` or `"discord"`) and `recipient` (chat ID or channel ID). Upon successful job completion, the cron engine MUST forward the resulting brain response to the Gateway Multiplexer to deliver to the configured recipient. If no target is specified, the output MUST be written to the application log.

#### Scenario: Cron job output delivered to Telegram
- GIVEN a job configured with target adapter `"telegram"` and recipient `"123456789"`
- WHEN the cron job executes and generates text output
- THEN the scheduler invokes the Gateway Multiplexer `Send(ctx, "telegram", "123456789", output)` and the message is delivered to Telegram

#### Scenario: Cron job without target logs output
- GIVEN a job configured without a target block
- WHEN the cron job executes successfully
- THEN the output is recorded in the scheduler log and no gateway send is triggered

---

## plugins (NEW)

### AGIS-M6-PLG-001: Plugin Manifest Schema (`plugin.json`)
The system MUST recognize external plugins placed in `$AGIS_HOME/plugins/<plugin_name>/` containing a `plugin.json` manifest. The manifest MUST validate against the following JSON schema:
- `name` (string, required): Unique lowercase identifier matching `^[a-z0-9-_]+$`.
- `version` (string, required): Semantic version string (e.g. `"1.0.0"`).
- `description` (string, optional): Short summary of plugin capabilities.
- `entrypoint` (string, optional): Executable or script name relative to plugin root for CLI tool bridges.
- `tools` (array of objects, optional): Tool definitions with `name`, `description`, and `parameters`.
- `skills` (array of strings, optional): Skill markdown file names located inside the plugin directory.
- `permissions` (array of strings, optional): Declared permission categories requested by the plugin.

#### Scenario: Valid plugin manifest parses successfully
- GIVEN a plugin directory containing a compliant `plugin.json`
- WHEN the Plugin Manager reads the manifest
- THEN all metadata, tools, and skills are extracted into memory structures without errors

#### Scenario: Malformed manifest rejected
- GIVEN a `plugin.json` missing the required `name` or `version` field
- WHEN the Plugin Manager inspects the directory
- THEN loading fails with a schema validation error and the plugin is marked invalid

---

### AGIS-M6-PLG-002: Plugin Manager Lifecycle (`Load`, `List`, `Enable`, `Disable`)
The Plugin Manager (`internal/plugins/`) MUST manage the discovery and lifecycle of plugins:
- `Load(dir string) error`: Scans the plugin root directory and loads all valid plugin manifests.
- `List() []PluginInfo`: Returns all discovered plugins, their status (`enabled` or `disabled`), version, and registered tools.
- `Enable(name string) error`: Activates a plugin and registers its tools and skills into AGIS registries.
- `Disable(name string) error`: Deactivates a plugin and unregisters its tools and skills.
- State (`enabled`/`disabled`) MUST persist across restarts in `$AGIS_HOME/plugins/state.json` or `config.yaml`.

#### Scenario: Discovered plugin enabled dynamically
- GIVEN a discovered plugin `"weather"` in disabled state
- WHEN `Enable("weather")` is executed
- THEN the plugin status becomes `enabled`, its state is persisted, and its tools become available in the tool registry

#### Scenario: Disabling plugin removes tools
- GIVEN an enabled plugin `"weather"` providing tool `"get_weather"`
- WHEN `Disable("weather")` is executed
- THEN `"get_weather"` is deregistered from the active tool registry and unavailable for subsequent turns

---

### AGIS-M6-PLG-003: Plugin Tool and Skill Registration
When a plugin is enabled:
- Its declared tools MUST be registered with the AGIS Tool Registry, executing the plugin's `entrypoint` executable via JSON-RPC or standard stdin/stdout command interface with arguments and receiving structured JSON results.
- Its declared skills (`.md` files) MUST be registered with the AGIS Skill Hub.
- Plugin tool executions MUST pass through `PolicyGuard` under the standard execution guardrails.

#### Scenario: Brain calls a plugin tool
- GIVEN an enabled plugin providing tool `"github_search"`
- WHEN the model emits a tool call for `"github_search"` with arguments
- THEN the tool runner executes the plugin entrypoint with the arguments and returns the standard tool output to the brain

---

## webhook (NEW)

### AGIS-M6-WBH-001: Webhook HTTP Listener Server
The system MUST provide a Webhook Server (`internal/webhook/`) that listens on a configured host and port (e.g. `127.0.0.1:8080`) and handles HTTP POST requests at the configured endpoint path (default `/webhook` or `/events`).
- The server MUST respond with `200 OK` on valid accepted payloads.
- The server MUST respond with `405 Method Not Allowed` on non-POST requests.
- The server MUST support graceful shutdown via `context.Context` without terminating active connections abruptly.

#### Scenario: HTTP POST request received on webhook path
- GIVEN a running Webhook HTTP server on port 8080 with path `"/webhook"`
- WHEN a valid HTTP POST request is sent to `http://127.0.0.1:8080/webhook`
- THEN the server processes the event and returns HTTP status `200 OK`

#### Scenario: HTTP GET request rejected
- GIVEN a running Webhook HTTP server
- WHEN an HTTP GET request is made to `"/webhook"`
- THEN the server returns HTTP status `405 Method Not Allowed`

---

### AGIS-M6-WBH-002: HMAC-SHA256 Signature Verification
When a webhook secret is configured in `config.yaml`, the server MUST verify the payload integrity and authenticity using HMAC-SHA256:
- The signature MUST be extracted from the `X-Hub-Signature-256` or `X-Signature` header (supporting `sha256=` prefix).
- The server MUST compute the HMAC-SHA256 of the raw request body using the configured secret and compare it to the header signature using constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- Requests with missing, invalid, or mismatched signatures MUST be rejected immediately with `401 Unauthorized` before reading or executing payload content.

#### Scenario: Valid HMAC-SHA256 signature accepted
- GIVEN a webhook configured with secret `"secret-token-123"`
- WHEN an HTTP POST arrives with body `{"event":"alert"}` and valid header `X-Hub-Signature-256: sha256=<valid_hmac>`
- THEN the server validates the signature and accepts the request

#### Scenario: Invalid signature rejected with 401
- GIVEN a webhook configured with secret `"secret-token-123"`
- WHEN an HTTP POST arrives with an invalid or tampered signature header
- THEN the server rejects the request with HTTP status `401 Unauthorized` and does not process the body

---

### AGIS-M6-WBH-003: Webhook Event Ingestion and Dispatch
Upon signature validation, the Webhook Server MUST parse the JSON event payload and dispatch it:
- The event MUST be dispatched to `core.Brain.Step` with a constructed prompt (e.g. `"Webhook event received: <payload>"`).
- The execution MUST use the configured `default_session_id` or an ephemeral session `webhook:<event_type>`.
- If configured with a gateway delivery target, the brain's response MUST be forwarded to the Gateway Multiplexer for outbound delivery.

#### Scenario: Webhook event triggers Brain turn
- GIVEN an accepted webhook event payload `{"alert": "high_cpu", "server": "app-01"}`
- WHEN the event is dispatched
- THEN `Brain.Step` executes the event prompt and logs or sends the response to the target gateway recipient

---

## config-loader (MODIFIED)

### AGIS-M6-CONF-003: Ecosystem Configuration Schema (Gateway, Cron, Plugins, Webhook)
The configuration loader in `internal/config/config.go` MUST support the following optional root configuration blocks:

```yaml
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
      session_id: "cron-health"
      target:
        adapter: "telegram"
        recipient: "123456"

plugins:
  enabled: false
  dir: "~/.agis/plugins"

webhook:
  enabled: false
  host: "127.0.0.1"
  port: 8080
  path: "/webhook"
  secret: ""
  default_session_id: "webhook-events"
  target:
    adapter: "telegram"
    recipient: "123456"
```

- All ecosystem blocks MUST be disabled by default (`enabled: false`).
- Missing fields MUST inherit documented safe defaults (`host: "127.0.0.1"`, `port: 8080`, `path: "/webhook"`, `plugins.dir: "$AGIS_HOME/plugins"`).
- Unmarshaling must handle environment variable expansions or missing files without panicking.

#### Scenario: Default configuration disables ecosystem blocks
- GIVEN an empty or minimal `config.yaml`
- WHEN `config.Load()` is invoked
- THEN `cfg.Gateway.Enabled`, `cfg.Cron.Enabled`, `cfg.Plugins.Enabled`, and `cfg.Webhook.Enabled` are all `false`

#### Scenario: Full ecosystem configuration parsed
- GIVEN a `config.yaml` containing complete `gateway`, `cron`, `plugins`, and `webhook` blocks
- WHEN `config.Load()` is invoked
- THEN all struct fields, job lists, allowlists, and secrets are populated accurately

---

## cli (MODIFIED)

### AGIS-M6-CLI-002: Ecosystem CLI Subcommands (`gateway`, `cron`, `plugins`, `webhook`)
The `cmd/agis/` CLI entry points MUST provide the following subcommands:
1. `agis gateway [run]`: Starts the Gateway multiplexer daemon with enabled chat adapters. Accepts `--config` flag. Listens for `SIGINT`/`SIGTERM` for graceful shutdown.
2. `agis cron [run|list]`:
   - `agis cron run`: Starts the cron scheduler in the background.
   - `agis cron list`: Prints all configured cron jobs with their schedule and target.
3. `agis plugins [list|enable|disable|inspect]`:
   - `agis plugins list`: Lists all discovered plugins, enabled status, and versions.
   - `agis plugins enable <name>`: Enables the specified plugin.
   - `agis plugins disable <name>`: Disables the specified plugin.
   - `agis plugins inspect <name>`: Displays manifest details, declared tools, and permissions.
4. `agis webhook [run]`: Starts the Webhook HTTP server listener daemon. Accepts `--port`, `--host`, `--path`, and `--config` flags.

All daemon subcommands MUST exit with code 0 on clean shutdown via signal and non-zero on fatal initialization failure.

#### Scenario: `agis gateway` runs and terminates on SIGINT
- GIVEN valid gateway configuration
- WHEN `agis gateway` is launched and sent `SIGINT`
- THEN the gateway shuts down all adapters cleanly and exits with status code 0

#### Scenario: `agis plugins list` displays plugin statuses
- GIVEN two plugins in the plugins directory (`"weather"` enabled, `"jira"` disabled)
- WHEN `agis plugins list` is executed
- THEN output lists both plugins with their correct names, versions, and enabled statuses

#### Scenario: `agis webhook` starts listener on custom port
- GIVEN flag `--port 9090`
- WHEN `agis webhook` runs
- THEN the server binds to `127.0.0.1:9090` and handles HTTP requests
