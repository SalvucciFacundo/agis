# Gateway Spec

## Purpose

Provide a multi-platform chat gateway multiplexer with adapters for Telegram and Discord, static allowlist enforcement, non-interactive sandbox policy execution, and session-bound conversational routing.

## Requirements

### Requirement AGIS-M6-GTW-001: Gateway Multiplexer and Adapter Port
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

### Requirement AGIS-M6-GTW-002: Telegram Adapter
The Telegram adapter MUST connect to the Telegram Bot API using the configured bot token. It MUST poll for updates or receive webhook updates, translate incoming Telegram messages into internal Gateway events, and deliver outbound replies via the Telegram Bot API `sendMessage` endpoint. Messages exceeding Telegram's 4096-character limit MUST be chunked or split cleanly using rune slicing without dropping content.

#### Scenario: Inbound Telegram message received
- GIVEN a configured and running Telegram adapter
- WHEN an authorized Telegram user sends `"Hello AGIS"`
- THEN the adapter transforms the payload into a standard Gateway message event with user ID, chat ID, and text content

#### Scenario: Long message split before sending
- GIVEN a brain response containing 5000 characters
- WHEN the Telegram adapter delivers the response via `Send`
- THEN the message is chunked into parts each under 4096 characters and delivered in sequence

### Requirement AGIS-M6-GTW-003: Discord Adapter
The Discord adapter MUST connect to the Discord Gateway via WebSocket or REST API using the configured bot token. It MUST listen for message creation events in permitted channels or direct messages, map Discord user IDs and channel IDs to internal Gateway events, and deliver outbound replies via Discord REST API. Messages exceeding Discord's 2000-character limit MUST be split cleanly into multiple sequential messages using rune slicing.

#### Scenario: Inbound Discord message received
- GIVEN a configured and running Discord adapter
- WHEN an authorized Discord user sends a message in a watched channel
- THEN the adapter converts the Discord event into an internal Gateway event with author ID, channel ID, and text content

#### Scenario: Outbound message chunking on Discord
- GIVEN a brain response containing 3000 characters
- WHEN the Discord adapter sends the message to a channel
- THEN the message is split into chunks of at most 2000 characters and sent sequentially

### Requirement AGIS-M6-GTW-004: User Allowlist Security Enforcement
Each gateway adapter MUST enforce a static user ID allowlist configured in `config.yaml`. Incoming messages from user IDs not present in the adapter's allowlist MUST be dropped or rejected immediately before allocating sessions or invoking the brain. Dropped unauthorized interactions MUST be logged with sender ID and platform name.

#### Scenario: Authorized user message accepted
- GIVEN a Telegram adapter with allowlist `["123456789"]`
- WHEN user `"123456789"` sends `"ping"`
- THEN the message passes allowlist validation and reaches the session router

#### Scenario: Unauthorized user message rejected
- GIVEN a Discord adapter with allowlist `["987654321"]`
- WHEN user `"111222333"` sends a message
- THEN the message is dropped, no brain invocation occurs, and an unauthorized access warning is logged

### Requirement AGIS-M6-GTW-005: Sandbox Policy Guard & Non-Interactive Auto-Deny
Gateway execution MUST run under the `sandbox` security posture without interactive human-in-the-loop prompts. When a tool call evaluated by `PolicyGuard` yields `DecisionAsk` (or requires interactive privilege escalation), the gateway brain runner MUST automatically deny the execution (`"blocked by policy"`), preventing remote deadlock. Persistent `always` allow rules configured in `policy.yaml` MUST continue to execute normally.

#### Scenario: Unapproved tool call auto-denied in gateway session
- GIVEN a gateway-routed turn triggering a tool call that requires approval (`DecisionAsk`)
- WHEN evaluated in the non-interactive gateway environment
- THEN the approver auto-denies the action, logs the denied attempt to the policy audit log, and informs the model the action was blocked

#### Scenario: Persistent allow rule executes in gateway session
- GIVEN a tool command with an explicit `always` allow rule in `policy.yaml`
- WHEN evaluated in a gateway session
- THEN the guard returns `DecisionAllow` and the tool executes successfully

### Requirement AGIS-M6-GTW-006: Session Routing and Brain Execution
The Gateway MUST maintain deterministic session mapping using `session.SessionManager`. Each platform user/channel combination MUST map to a unique session key (e.g. `gateway:telegram:<chat_id>` or `gateway:discord:<channel_id>`). Inbound messages MUST be routed to `core.Brain.Step`, streaming text chunks and aggregating the final response to send back via the originating adapter.

#### Scenario: Session continuity across consecutive messages
- GIVEN an existing gateway session for Telegram chat ID `445566`
- WHEN the user sends a second message within the same chat
- THEN the brain receives the previous conversation history from `SessionManager` and appends the new turn


gateway (MODIFIED)

### Requirement AGIS-M9-GTW-001: Telegram Photo and Voice Ingestion
The Telegram adapter in `internal/gateway/telegram.go` MUST support downloading and processing inbound photos and voice notes:
1. **Photos**: When an update contains a `photo` array, the adapter MUST select the largest resolution entry, resolve its file path via the Telegram `getFile` endpoint, and download the binary payload.
2. **Voice & Audio Notes**: When an update contains a `voice` or `audio` payload, the adapter MUST resolve the file path, download the audio bytes, and invoke `core.Transcriber.Transcribe` to generate a text transcript.
3. The adapter MUST populate `MessageEvent.Attachments` with the downloaded binary payload and MIME metadata.

#### Scenario: Inbound Telegram photo processed
- GIVEN a Telegram user sending a photo with caption `"What's in this image?"`
- WHEN the adapter processes the update
- THEN the image is downloaded, attached to the message event, and passed to the brain

#### Scenario: Inbound Telegram voice note transcribed
- GIVEN a Telegram user sending a voice message
- WHEN the adapter downloads the audio and invokes `Transcriber`
- THEN the audio is transcribed to text, attached to the event, and processed by the brain

---

### Requirement AGIS-M9-GTW-002: Discord Media Ingestion
The Discord adapter in `internal/gateway/discord.go` MUST support downloading attachments:
1. Inspect inbound message attachments for `image/*` and `audio/*` content types.
2. Download media bytes from the Discord CDN URL with bounded timeouts.
3. If the attachment is audio, invoke `core.Transcriber` to produce text transcripts.
4. Populate `MessageEvent.Attachments` with the downloaded media payload.

#### Scenario: Inbound Discord image attachment processed
- GIVEN a Discord message with an image attachment
- WHEN the adapter downloads the attachment
- THEN the image is validated and attached to the event

---

### Requirement AGIS-M9-GTW-003: Media Size and MIME Guardrails
Each gateway adapter MUST enforce strict media validation guardrails before processing:
1. **Size Limits**: Images MUST NOT exceed 10MB (`10 * 1024 * 1024` bytes). Audio payloads MUST NOT exceed 25MB (`25 * 1024 * 1024` bytes). Oversized payloads MUST be dropped or rejected with a warning.
2. **MIME Sniffing**: The adapter MUST inspect magic bytes via `http.DetectContentType` to ensure payloads match genuine allowed MIME formats (`image/png`, `image/jpeg`, `image/webp`, `image/gif`, `audio/ogg`, `audio/wav`, `audio/mpeg`). Executable or unrecognized payloads MUST be rejected fail-closed.

#### Scenario: Oversized image rejected
- GIVEN an inbound image payload of 15MB
- WHEN evaluated by gateway media guards
- THEN the download is rejected and an oversized media warning is logged

