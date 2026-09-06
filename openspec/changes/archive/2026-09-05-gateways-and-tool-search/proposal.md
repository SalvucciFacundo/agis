# Proposal: gateways-and-tool-search

## 1. Intent
Introduce additional messaging gateways (Slack and WhatsApp) to expand AGIS's chat platform support. Additionally, implement dynamic tool search and lazy schema loading to prevent context bloat and token exhaustion when many tools (e.g., via MCP plugins) are registered.

## 2. Scope
### 2.1. Additional Messaging Gateways (`internal/gateway`)
- **Slack Adapter (`internal/gateway/slack.go`)**:
  - Webhook/Events API and Socket Mode listener integration.
  - Constant-time HMAC signature verification (`X-Slack-Signature` with `v0:` prefix and timestamp check).
  - User allowlist filtering using `gateway.IsAllowed`.
  - Session mapping via `gateway:slack:<channelID>:<userID>`.
  - 4000-character chunking, thread support, and media attachment ingestion.
- **WhatsApp Adapter (`internal/gateway/whatsapp.go`)**:
  - Meta Cloud API / Webhook bridge integration.
  - HMAC payload verification (`X-Hub-Signature-256`) and verification token validation.
  - User allowlist filtering.
  - Session mapping via `gateway:whatsapp:<phone>`.
  - 4096-character chunking and voice note transcription (leveraging existing Whisper/audio ports).
- **Multiplexer Integration**: Wire both adapters into `gateway.Multiplexer` alongside Telegram and Discord.

### 2.2. Dynamic Tool Search & Lazy Schema Loading (`internal/tools/` & `internal/core/`)
- **Threshold-based Pruning**: If the total number of tools exceeds `tools.tool_search.threshold` (default 8) and `enabled` is true, the Brain will default to exposing only core tools (`web_search`, `delegate_task`, `shell`).
- **Bridge Tools**:
  - `tool_search(query string, category string)`: Searches the internal registry and returns tool names and short descriptions.
  - `load_tool(name string)`: Dynamically injects the full schema of the requested tool into the active turn's `ChatRequest.Tools` list, making it available for subsequent LLM rounds.

### 2.3. Configuration, Security & Doctor Probes
- **`internal/config/config.go`**: Add `gateway.slack`, `gateway.whatsapp`, and `tools.tool_search` structures. Secrets must use masking for logging.
- **`internal/doctor/`**: Introduce diagnostic probes to verify Slack/WhatsApp credentials and webhook endpoints, and to check the tool search status.

## 3. Affected Areas
- `internal/gateway/`: Adapter implementations and multiplexer wiring.
- `internal/tools/` & `internal/core/brain.go`: Tool registry expansion, active tool filtering logic, and new native tools (`tool_search`, `load_tool`).
- `internal/config/config.go`: Expanded structs for Gateway and Tools.
- `internal/doctor/`: Additional health checks.

## 4. Risks & Mitigations
- **Security Bypass on Webhooks**: A failure in HMAC verification could allow forged messages. 
  - *Mitigation*: Strictly enforce constant-time string comparison (`crypto/subtle`) and timestamp freshness (replay attack prevention).
- **Prompt Bloat Despite Pruning**: Dynamically loading too many tools in a single turn could still exhaust tokens.
  - *Mitigation*: The LLM should only load tools it intends to use. The max rounds limit (`maxToolRounds`) naturally bounds runaway loading.
- **Voice Note Processing Latency**: Transcribing long WhatsApp voice notes could delay responsiveness.
  - *Mitigation*: Use streaming processing where possible or enforce maximum audio lengths (already governed by `MaxAudioSizeMB` config).

## 5. Rollback Strategy
- Adapter implementations are isolated. If they fail, they can be disabled via `config.yaml` (`gateway.slack.enabled: false`, `gateway.whatsapp.enabled: false`).
- Tool search logic is guarded by `tools.tool_search.enabled`. If toggled off, AGIS reverts to the standard behavior (advertising all tools).

## 6. Success Criteria
- Slack and WhatsApp adapters securely receive, authenticate, parse, and reply to messages.
- The Multiplexer handles routing across 4 separate platforms concurrently.
- `tool_search` activates automatically when tools exceed the threshold, successfully hiding non-core tools until `load_tool` brings them into the active prompt context.
- Doctor commands correctly validate the new configurations.
