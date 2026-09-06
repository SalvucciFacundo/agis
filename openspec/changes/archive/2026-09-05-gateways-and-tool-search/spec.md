# Specification: Messaging Gateways & Dynamic Tool Search (gateways-and-tool-search)

## Purpose

Expand AGIS chat gateway integrations by implementing production-grade adapters for Slack (Events API / Webhooks) and WhatsApp (Meta Cloud API / Webhooks), complete with constant-time HMAC verification, user allowlist enforcement, session mapping, media and voice note ingestion, and message chunking. Furthermore, introduce dynamic tool search and lazy schema loading (`tool_search`, `load_tool`) to mitigate prompt context bloat and token exhaustion when extensive toolsets (e.g., MCP servers, plugins) are loaded.

---

## 1. Slack Messaging Gateway (`internal/gateway/slack.go`)

### Requirement GTW-SLK-001: Slack Adapter Contract & Lifecycle
The system MUST provide a Slack gateway adapter in `internal/gateway/slack.go` implementing the `gateway.Adapter` interface (`Name() string`, `Start(ctx context.Context) error`, `Stop() error`, `Send(ctx context.Context, target string, msg string) error`).
- `Name()` MUST return `"slack"`.
- `Start(ctx)` MUST start an HTTP server listening on the configured `ListenAddr` (default `:3002` or shared webhook mux) handling Slack Events API callbacks, or initiate a Socket Mode connection when configured.
- `Stop()` MUST gracefully terminate HTTP listeners and wait for active inbound message processing to drain.
- `Send(ctx, target, msg)` MUST post messages to the specified Slack channel or direct message target via Slack Web API `chat.postMessage` (`https://slack.com/api/chat.postMessage`).

#### Scenario: Slack adapter starts and stops cleanly
- GIVEN a valid `SlackGatewayConfig` with `Enabled: true` and `ListenAddr: ":3002"`
- WHEN `Start(ctx)` is invoked on the Slack adapter
- THEN an HTTP listener begins accepting incoming POST requests on `:3002`
- AND WHEN `Stop()` is invoked
- THEN the listener shuts down gracefully with zero leaked goroutines

---

### Requirement GTW-SLK-002: Slack Signature Verification & Security
The Slack adapter MUST verify the authenticity of all inbound HTTP requests using constant-time HMAC-SHA256 signature verification prior to parsing request bodies:
1. **Timestamp Freshness**: The adapter MUST extract the `X-Slack-Request-Timestamp` header. If the timestamp differs from the current UNIX time by more than 300 seconds (5 minutes), the adapter MUST reject the request with HTTP 400 Bad Request to prevent replay attacks.
2. **Signature Computation**: The adapter MUST compute `HMAC-SHA256` of `"v0:" + timestamp + ":" + raw_body` using `SlackGatewayConfig.SigningSecret`.
3. **Constant-Time Comparison**: The adapter MUST compare the computed signature formatted as `"v0=" + hex(hmac)` against the `X-Slack-Signature` header using `crypto/subtle.ConstantTimeCompare`. Mismatched signatures MUST result in HTTP 401 Unauthorized.

#### Scenario: Valid Slack signature accepted
- GIVEN an inbound HTTP POST with valid `X-Slack-Request-Timestamp` (< 5 min old) and matching `X-Slack-Signature`
- WHEN the Slack adapter verifies the request
- THEN verification succeeds and the request body is dispatched for event handling

#### Scenario: Expired Slack timestamp rejected
- GIVEN an inbound HTTP POST with `X-Slack-Request-Timestamp` from 10 minutes ago
- WHEN the Slack adapter checks timestamp freshness
- THEN the request is rejected with HTTP 400 and an error is logged

#### Scenario: Forged Slack signature rejected
- GIVEN an inbound HTTP POST with an invalid or tampered `X-Slack-Signature`
- WHEN the Slack adapter validates the signature using `crypto/subtle.ConstantTimeCompare`
- THEN the request is rejected with HTTP 401 Unauthorized and processing halts immediately

---

### Requirement GTW-SLK-003: Slack Challenge & Event Handling
The Slack adapter MUST handle Slack Events API payload types:
1. **URL Verification**: When the inbound payload contains `{"type": "url_verification", "challenge": "<token>"}`, the adapter MUST immediately respond with HTTP 200 and JSON body `{"challenge": "<token>"}`.
2. **Event Callback (`type: "event_callback"`)**:
   - The adapter MUST process `message` and `app_mention` events.
   - The adapter MUST ignore events from bots (e.g. `bot_id != ""` or `subtype == "bot_message"`) to prevent infinite message loops.
   - The adapter MUST extract `user` (sender ID), `channel` (channel ID), `text` (message body), and `thread_ts` (thread timestamp).
3. **Allowlist Validation**: The adapter MUST verify `user` against `SlackGatewayConfig.AllowedUsers` using `gateway.IsAllowed`. Unauthorized senders MUST be dropped with an unauthorized log warning.
4. **Session Mapping**: The adapter MUST map the conversation to session key `gateway:slack:<channelID>:<userID>` (or `gateway:slack:<channelID>` for thread-bound channels) and dispatch a normalized `gateway.MessageEvent` to the registered `Handler`.

#### Scenario: Slack URL verification handshake
- GIVEN a Slack Events API URL verification request with challenge `"test_challenge_123"`
- WHEN the Slack adapter endpoint processes the POST request
- THEN it responds with HTTP 200 and `{"challenge": "test_challenge_123"}`

#### Scenario: Bot messages ignored
- GIVEN an incoming Slack message event with `bot_id: "B12345"`
- WHEN the Slack adapter parses the event
- THEN the message is ignored and no event is dispatched to the multiplexer

#### Scenario: Authorized user message routed to session
- GIVEN an incoming Slack message from authorized user `"U12345"` in channel `"C67890"` with text `"Summarize release notes"`
- WHEN the Slack adapter processes the event
- THEN a `MessageEvent` with `Adapter: "slack"`, `UserID: "U12345"`, `ChatID: "C67890"`, and `Content: "Summarize release notes"` is dispatched to the handler

---

### Requirement GTW-SLK-004: Slack Message Chunking & Outbound Delivery
The Slack adapter MUST enforce outbound character limits when sending replies via `Send`:
1. **Chunking**: Outbound messages exceeding `SlackMaxMessageLength` (4000 runes) MUST be cleanly chunked into sequential messages using rune slicing (`gateway.SplitMessage(msg, 4000)`).
2. **Delivery**: Each chunk MUST be transmitted via `chat.postMessage` using the configured `BotToken`.
3. **Thread Support**: If the original inbound event contained a `thread_ts`, outbound replies SHOULD target the same thread by setting `thread_ts` in the API payload.

#### Scenario: Long response chunked under 4000 characters
- GIVEN an outbound response string containing 7500 runes
- WHEN `Send(ctx, "C67890", msg)` is called on the Slack adapter
- THEN the message is split into 2 chunks (4000 runes and 3500 runes) and delivered sequentially to channel `"C67890"`

---

## 2. WhatsApp Messaging Gateway (`internal/gateway/whatsapp.go`)

### Requirement GTW-WHA-001: WhatsApp Adapter Contract & Lifecycle
The system MUST provide a WhatsApp gateway adapter in `internal/gateway/whatsapp.go` implementing the `gateway.Adapter` interface (`Name() string`, `Start(ctx context.Context) error`, `Stop() error`, `Send(ctx context.Context, target string, msg string) error`).
- `Name()` MUST return `"whatsapp"`.
- `Start(ctx)` MUST start an HTTP webhook listener on `ListenAddr` (default `:3003` or shared mux) handling Meta Webhook GET and POST requests.
- `Stop()` MUST gracefully shut down the webhook listener and drain active operations.
- `Send(ctx, target, msg)` MUST transmit messages to the recipient phone number via the Meta Graph API WhatsApp Cloud endpoint (`https://graph.facebook.com/v21.0/<PhoneNumberID>/messages`) using `APIToken`.

#### Scenario: WhatsApp adapter starts and accepts webhooks
- GIVEN a valid `WhatsAppGatewayConfig` with `Enabled: true` and `ListenAddr: ":3003"`
- WHEN `Start(ctx)` is invoked
- THEN an HTTP server starts listening for Meta webhook requests on `:3003`

---

### Requirement GTW-WHA-002: WhatsApp Webhook Verification & Security
The WhatsApp adapter MUST enforce security on both verification (GET) and inbound event (POST) requests:
1. **Webhook Verification (GET)**:
   - When Meta sends a verification request with query parameters `hub.mode`, `hub.verify_token`, and `hub.challenge`, the adapter MUST verify that `hub.mode == "subscribe"` and `hub.verify_token` matches `WhatsAppGatewayConfig.VerifyToken` using `crypto/subtle.ConstantTimeCompare`.
   - On match, the adapter MUST respond with HTTP 200 and the plain `hub.challenge` string. Mismatches MUST return HTTP 403 Forbidden.
2. **Payload Signature Verification (POST)**:
   - The adapter MUST extract the `X-Hub-Signature-256` header (formatted as `sha256=<hex>`).
   - The adapter MUST compute the `HMAC-SHA256` of the raw request body using `WhatsAppGatewayConfig.AppSecret`.
   - The adapter MUST compare signatures using `crypto/subtle.ConstantTimeCompare`. If signatures do not match or the header is missing, the adapter MUST return HTTP 401 Unauthorized.

#### Scenario: Webhook setup verification succeeds
- GIVEN a Meta GET webhook request with `hub.mode=subscribe`, `hub.verify_token=my_secret_verify_token`, and `hub.challenge=1158201444`
- WHEN `WhatsAppGatewayConfig.VerifyToken` is `"my_secret_verify_token"`
- THEN the adapter returns HTTP 200 with body `"1158201444"`

#### Scenario: Inbound webhook POST with valid HMAC signature
- GIVEN an inbound POST request containing WhatsApp message JSON with valid `X-Hub-Signature-256` computed from `AppSecret`
- WHEN the WhatsApp adapter validates the request
- THEN the signature check passes and message processing continues

#### Scenario: Inbound webhook POST with invalid HMAC signature
- GIVEN an inbound POST request with tampered payload or incorrect `X-Hub-Signature-256`
- WHEN the WhatsApp adapter validates the signature
- THEN the adapter rejects the request with HTTP 401 Unauthorized and halts processing

---

### Requirement GTW-WHA-003: WhatsApp Message Parsing, Allowlist & Media Handling
The WhatsApp adapter MUST parse inbound message structures from Meta Cloud API payloads (`entry[].changes[].value.messages[]`):
1. **Allowlist Filtering**: The sender's phone number (`from`) MUST be checked against `WhatsAppGatewayConfig.AllowedUsers` using `gateway.IsAllowed`. Unauthorized senders MUST be dropped immediately with a logged warning.
2. **Text Messages (`type: "text"` )**: Extract `text.body`, construct `MessageEvent` with `Adapter: "whatsapp"`, `UserID: from`, `ChatID: from`, and dispatch to `Handler`.
3. **Voice Notes / Audio (`type: "audio"` or `type: "voice"`)**:
   - Extract audio attachment metadata (`id`, `mime_type`).
   - Query Meta Graph API `https://graph.facebook.com/v21.0/<media_id>` using `APIToken` to obtain the secure media download URL.
   - Download audio bytes with timeout and size enforcement (`<= 25MB`).
   - Transcribe audio bytes via `core.Transcriber.Transcribe` (Whisper) and populate `MessageEvent.Content` with transcribed text and `MessageEvent.Attachments` with audio metadata.
4. **Session Mapping**: Map conversation to session key `gateway:whatsapp:<phone>`.

#### Scenario: Inbound text message processed
- GIVEN an inbound WhatsApp message from authorized number `"+15551234567"` with body `"How are you?"`
- WHEN the WhatsApp adapter processes the webhook event
- THEN a `MessageEvent` is dispatched to the multiplexer with `UserID: "+15551234567"`, `ChatID: "+15551234567"`, and `Content: "How are you?"`

#### Scenario: Inbound voice note transcribed
- GIVEN an inbound WhatsApp voice message from authorized number `"+15551234567"` with media ID `"m_998877"`
- WHEN the adapter downloads the audio and invokes `core.Transcriber`
- THEN the audio is transcribed to `"Hello AGIS"` and passed as `MessageEvent.Content` to the brain

---

### Requirement GTW-WHA-004: WhatsApp Message Chunking & Outbound Delivery
The WhatsApp adapter MUST deliver outbound replies via Meta Cloud API:
1. **Chunking**: Outbound messages exceeding `WhatsAppMaxMessageLength` (4096 runes) MUST be split cleanly using rune slicing (`gateway.SplitMessage(msg, 4096)`).
2. **Delivery**: Each chunk MUST be sent as a JSON POST payload to `https://graph.facebook.com/v21.0/<PhoneNumberID>/messages` with `messaging_product: "whatsapp"`, `recipient_type: "individual"`, `to: target`, and `type: "text"`.
3. **Authentication**: Requests MUST carry HTTP header `Authorization: Bearer <APIToken>`.

#### Scenario: Outbound WhatsApp message delivered
- GIVEN an outbound response text of 500 characters directed to `"+15551234567"`
- WHEN `Send(ctx, "+15551234567", text)` is executed
- THEN an HTTP POST is made to Meta Graph API with bearer token and recipient `"+15551234567"`

---

## 3. Dynamic Tool Search & Lazy Schema Loading (`internal/tools/` & `internal/core/`)

### Requirement TLS-SRC-001: Tool Metadata Registry & Indexing
The tool system MUST maintain metadata for all registered tools, enabling lightweight searching without serializing full JSON schemas into the prompt:
1. Every `core.ToolRunner` MUST expose metadata or name, description, and backend category.
2. The registry MUST maintain a searchable index of `{Name: string, Description: string, Category: string, Backend: string}` for all available tools.

#### Scenario: Tool inventory indexed
- GIVEN 15 registered tools across local, web, mcp, and subagents backends
- WHEN the tool search registry is queried
- THEN all 15 tool summaries are indexed with name, description, and category

---

### Requirement TLS-SRC-002: `tool_search` Native Tool Contract
The system MUST provide a native tool named `tool_search` implementing `core.ToolRunner`:
- **Tool Name**: `"tool_search"`
- **Backend**: `"internal"`
- **Description**: `"Search for available tools by keyword or category when needed. Returns matching tool names and short descriptions."`
- **Input Schema**:
  - `query` (string, required): Search keywords to match against tool names and descriptions.
  - `category` (string, optional): Filter by tool category (e.g. `"web"`, `"fs"`, `"git"`, `"mcp"`, `"subagents"`).
- **Output Schema**: JSON array of matching tool summaries:
  ```json
  [
    {
      "name": "mcp_github_create_issue",
      "description": "Create a new issue on a GitHub repository",
      "category": "mcp",
      "backend": "mcp"
    }
  ]
  ```
- **Error Handling**: If `query` is empty and `category` is empty, return an error indicating at least one search parameter is required.

#### Scenario: Search tools by keyword
- GIVEN tools `["web_search", "web_fetch", "mcp_slack_post", "mcp_github_create_issue"]`
- WHEN `tool_search` is called with `{"query": "github"}`
- THEN the tool returns `[{"name": "mcp_github_create_issue", "description": "Create a new issue on a GitHub repository", "category": "mcp", "backend": "mcp"}]`

---

### Requirement TLS-SRC-003: `load_tool` Native Tool Contract
The system MUST provide a native tool named `load_tool` implementing `core.ToolRunner`:
- **Tool Name**: `"load_tool"`
- **Backend**: `"internal"`
- **Description**: `"Load the full schema and enable a specific tool for the current conversation turn."`
- **Input Schema**:
  - `name` (string, required): The exact name of the tool to load.
- **Output Schema**: Confirmation JSON containing loaded tool details:
  ```json
  {
    "status": "loaded",
    "name": "mcp_github_create_issue",
    "description": "Create a new issue on a GitHub repository"
  }
  ```
- **Behavior**: Upon execution within `Brain.Step`, the requested tool's full definition MUST be added to the active turn's loaded tool set, ensuring it is advertised in subsequent LLM rounds of the same turn.
- **Error Handling**: If the tool name does not exist in the registered inventory, return an error `"tool '<name>' not found"`.

#### Scenario: Tool dynamically loaded
- GIVEN tool `mcp_jira_create_ticket` is currently unadvertised in prompt context
- WHEN the model calls `load_tool` with `{"name": "mcp_jira_create_ticket"}`
- THEN `load_tool` activates the tool definition for subsequent tool-calling rounds in the current turn
- AND returns confirmation `{"status": "loaded", "name": "mcp_jira_create_ticket"}`

---

### Requirement TLS-SRC-004: Threshold-Based Schema Pruning in Brain
When `tools.tool_search.enabled: true` and the total registered tool count exceeds `tools.tool_search.threshold` (default: 8):
1. **Initial Advertised Tools**: The brain MUST prune the initial `ChatRequest.Tools` list to include ONLY:
   - **Core Tools**: `web_search`, `web_fetch`, `delegate_task`, and `local` (or default core toolset).
   - **Search Bridge Tools**: `tool_search` and `load_tool`.
2. **Hidden Tools**: All other tools (e.g. MCP tools, external plugins, optional extensions) MUST NOT be advertised in the initial LLM prompt turn.
3. **Dynamic Turn Expansion**: When `load_tool(name)` succeeds during turn execution, the Brain MUST append the full definition of `name` to the turn's active tool list for all remaining rounds of that turn.
4. **Disabled Fallback**: If `tools.tool_search.enabled` is `false` or total tool count is `<= threshold`, all registered tools MUST be advertised upfront without pruning.

#### Scenario: Tool pruning activated above threshold
- GIVEN 20 tools registered, `tool_search.enabled: true`, and `tool_search.threshold: 8`
- WHEN a new user turn begins in `Brain.Step`
- THEN `ChatRequest.Tools` contains only Core Tools + `tool_search` + `load_tool` (total <= 6 tools)
- AND non-core MCP tools are excluded from initial prompt context

#### Scenario: Non-core tool discovered and loaded mid-turn
- GIVEN tool pruning is active and a user asks `"Create a Jira ticket"`
- WHEN the model calls `tool_search(query: "jira")` -> receives `mcp_jira_create_ticket`
- AND the model calls `load_tool(name: "mcp_jira_create_ticket")`
- THEN round 3 of `Brain.Step` includes `mcp_jira_create_ticket` in `ChatRequest.Tools`
- AND the model successfully invokes `mcp_jira_create_ticket` in round 3

#### Scenario: Tool search disabled
- GIVEN `tools.tool_search.enabled: false` and 20 registered tools
- WHEN `Brain.Step` prepares `ChatRequest.Tools`
- THEN all 20 tools are advertised upfront in the initial prompt

---

## 4. Configuration Schema & Security (`internal/config/`)

### Requirement CFG-GTW-001: Gateway & Tool Search Configuration Structs
`internal/config/config.go` MUST define configuration structs for Slack, WhatsApp, and Tool Search:

```go
type GatewayConfig struct {
    Enabled  bool           `yaml:"enabled"`
    Telegram TelegramConfig `yaml:"telegram"`
    Discord  DiscordConfig  `yaml:"discord"`
    Slack    SlackConfig    `yaml:"slack"`
    WhatsApp WhatsAppConfig `yaml:"whatsapp"`
}

type SlackConfig struct {
    Enabled       bool     `yaml:"enabled"`
    BotToken      string   `yaml:"bot_token"`
    SigningSecret string   `yaml:"signing_secret"`
    AllowedUsers  []string `yaml:"allowed_users"`
    ListenAddr    string   `yaml:"listen_addr"` // default: ":3002"
}

type WhatsAppConfig struct {
    Enabled       bool     `yaml:"enabled"`
    APIToken      string   `yaml:"api_token"`
    PhoneNumberID string   `yaml:"phone_number_id"`
    VerifyToken   string   `yaml:"verify_token"`
    AppSecret     string   `yaml:"app_secret"`
    AllowedUsers  []string `yaml:"allowed_users"`
    ListenAddr    string   `yaml:"listen_addr"` // default: ":3003"
}

type ToolsConfig struct {
    Enabled    bool             `yaml:"enabled"`
    Local      LocalConfig      `yaml:"local"`
    Docker     DockerConfig     `yaml:"docker"`
    SSH        SSHConfig        `yaml:"ssh"`
    Web        WebConfig        `yaml:"web"`
    ToolSearch ToolSearchConfig `yaml:"tool_search"`
}

type ToolSearchConfig struct {
    Enabled   bool `yaml:"enabled"`
    Threshold int  `yaml:"threshold"` // default: 8
}
```

- **Built-in Defaults**:
  - `Gateway.Slack.ListenAddr`: `":3002"`
  - `Gateway.WhatsApp.ListenAddr`: `":3003"`
  - `Tools.ToolSearch.Enabled`: `false` (opt-in; can be enabled in config)
  - `Tools.ToolSearch.Threshold`: `8`

#### Scenario: Default configuration initialized
- GIVEN an empty or minimal config
- WHEN `config.Load` executes
- THEN `Tools.ToolSearch.Threshold` defaults to `8`, `Slack.ListenAddr` defaults to `":3002"`, and `WhatsApp.ListenAddr` defaults to `":3003"`

---

### Requirement CFG-SEC-001: Secret Masking & Accessors
1. **Secret Masking (`internal/config/mask.go`)**:
   - `MaskSecrets` MUST mask the following fields with `"[MASKED]"`:
     - `Gateway.Slack.BotToken`
     - `Gateway.Slack.SigningSecret`
     - `Gateway.WhatsApp.APIToken`
     - `Gateway.WhatsApp.VerifyToken`
     - `Gateway.WhatsApp.AppSecret`
2. **Accessors (`internal/config/accessor.go`)**:
   - `Get` and `Set` MUST support dot-notation path keys:
     - `gateway.slack.enabled`, `gateway.slack.bot_token`, `gateway.slack.signing_secret`, `gateway.slack.listen_addr`
     - `gateway.whatsapp.enabled`, `gateway.whatsapp.api_token`, `gateway.whatsapp.phone_number_id`, `gateway.whatsapp.verify_token`, `gateway.whatsapp.app_secret`, `gateway.whatsapp.listen_addr`
     - `tools.tool_search.enabled`, `tools.tool_search.threshold`

#### Scenario: Secret masking hides Slack and WhatsApp credentials
- GIVEN a config with `gateway.slack.bot_token: "xoxb-123456"` and `gateway.whatsapp.app_secret: "secret_abc"`
- WHEN `config.MaskSecrets` is called
- THEN the resulting clone contains `"[MASKED]"` for both fields

#### Scenario: Dynamic accessor updates tool search threshold
- GIVEN a loaded config
- WHEN `config.Set(cfg, "tools.tool_search.threshold", "12")` is executed
- THEN `cfg.Tools.ToolSearch.Threshold` equals `12`

---

## 5. Diagnostic Probes (`internal/doctor/`)

### Requirement DOC-GTW-001: Slack Gateway Diagnostic Probe
`internal/doctor/` MUST include a `checkSlackGateway` probe (`"gateway_slack"`):
- If `Gateway.Slack.Enabled` is `false`:
  - Status: `StatusPass`
  - Message: `"Slack gateway disabled"`
- If `Gateway.Slack.Enabled` is `true`:
  - Verify `BotToken` is non-empty. If empty, return `StatusFail` (`"Slack bot token is missing"`).
  - Verify `SigningSecret` is non-empty. If empty, return `StatusFail` (`"Slack signing secret is missing"`).
  - Check `AllowedUsers`. If empty, return `StatusWarn` (`"Slack allowlist is empty (fail-closed, all messages will be rejected)"`).
  - If all checks pass, return `StatusPass` with configured details (listen address, allowlist count).

#### Scenario: Slack doctor check passes with complete config
- GIVEN `gateway.slack.enabled: true`, `bot_token: "xoxb-..."`, `signing_secret: "sec-..."`, `allowed_users: ["U123"]`
- WHEN `doctor.Run` executes
- THEN the `"gateway_slack"` check returns `StatusPass`

#### Scenario: Slack doctor check fails when token is missing
- GIVEN `gateway.slack.enabled: true` with empty `bot_token`
- WHEN `doctor.Run` executes
- THEN the `"gateway_slack"` check returns `StatusFail` with `"Slack bot token is missing"`

---

### Requirement DOC-GTW-002: WhatsApp Gateway Diagnostic Probe
`internal/doctor/` MUST include a `checkWhatsAppGateway` probe (`"gateway_whatsapp"`):
- If `Gateway.WhatsApp.Enabled` is `false`:
  - Status: `StatusPass`
  - Message: `"WhatsApp gateway disabled"`
- If `Gateway.WhatsApp.Enabled` is `true`:
  - Verify `APIToken` is non-empty. If empty, return `StatusFail` (`"WhatsApp API token is missing"`).
  - Verify `PhoneNumberID` is non-empty. If empty, return `StatusFail` (`"WhatsApp phone number ID is missing"`).
  - Verify `VerifyToken` is non-empty. If empty, return `StatusFail` (`"WhatsApp verify token is missing"`).
  - Verify `AppSecret` is non-empty. If empty, return `StatusFail` (`"WhatsApp app secret is missing"`).
  - Check `AllowedUsers`. If empty, return `StatusWarn` (`"WhatsApp allowlist is empty (fail-closed, all messages will be rejected)"`).
  - If all checks pass, return `StatusPass` with listen address and phone number ID details.

#### Scenario: WhatsApp doctor check passes
- GIVEN `gateway.whatsapp.enabled: true` with all required tokens and phone number ID configured
- WHEN `doctor.Run` executes
- THEN the `"gateway_whatsapp"` check returns `StatusPass`

---

### Requirement DOC-TLS-001: Tool Search Diagnostic Probe
`internal/doctor/` MUST include a `checkToolSearch` probe (`"tool_search"`):
- If `Tools.ToolSearch.Enabled` is `false`:
  - Status: `StatusPass`
  - Message: `"Dynamic tool search disabled (all registered tools advertised upfront)"`
- If `Tools.ToolSearch.Enabled` is `true`:
  - Validate `Threshold > 0`. If `Threshold <= 0`, return `StatusWarn` (`"Tool search threshold is <= 0, defaulting to 8"`).
  - Status: `StatusPass`
  - Message: `"Dynamic tool search enabled (threshold: <threshold>)"`

#### Scenario: Tool search doctor check passes
- GIVEN `tools.tool_search.enabled: true` and `tools.tool_search.threshold: 10`
- WHEN `doctor.Run` executes
- THEN the `"tool_search"` check returns `StatusPass` with `"Dynamic tool search enabled (threshold: 10)"`
