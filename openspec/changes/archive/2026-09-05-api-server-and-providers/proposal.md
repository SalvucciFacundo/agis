# Proposal: OpenAI-Compatible REST API Server & Expanded LLM Providers

## Intent
This change introduces a standalone OpenAI-compatible REST API server (`agis serve`) to allow third-party clients (e.g., LobeChat, typingmind, custom frontends) to interact with AGIS. Instead of simple pass-through to LLMs, these API requests will execute the full AGIS cognitive loop (memory, tools, skills, policy guard) by mapping the request to `core.Brain.Step`.
Additionally, this change significantly expands native LLM provider configurations (Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, Together AI, Cohere, OpenRouter) out-of-the-box in the `internal/adapters/llm/` factory, and adds server observability to the `internal/doctor` subsystem.

## Scope
1. **API Server (`internal/server/`)**:
   - `POST /v1/chat/completions`: Supports standard and streaming (`text/event-stream`) completions, mapping OpenAI's `messages` payload to AGIS interactions. Preserves sessions via the `user` field or `X-Session-ID` header.
   - `GET /v1/models`: Enumerates available models and active providers.
   - `GET /healthz` & `GET /v1/health`: Health-check endpoints.
   - **Authentication**: Optional Bearer token validation against `ServerConfig.APIKey`.
   - **CLI Integration**: A new `agis serve` (or `agis api`) command with flags `-host`, `-port`, `-api-key`, `-cors`.
   - **Graceful Shutdown**: Context-aware signal handling for stopping the HTTP server.

2. **Provider Factory Expansion (`internal/adapters/llm/`)**:
   - Enhance `NewProvider` to resolve predefined base URLs and default configurations for Anthropic, Google Gemini, DeepSeek, Groq, Mistral, xAI, Together AI, Cohere, and OpenRouter.
   - Most providers can map to the existing `OpenAI` adapter by injecting the correct `BaseURL`.
   - Anthropic and Cohere might require dedicated minimal adapters (or Anthropic-to-OpenAI translation) depending on their API shape, but native adapters are preferred for correctness if they deviate from the OpenAI v1 spec.

3. **Observability & Configuration**:
   - Add `ServerConfig` block to `internal/config/config.go` and the `Config` root.
   - Add `internal/doctor` probes to check server port bindability, API key presence, and basic reachability of the configured active/fallback providers.

## Affected Areas
- **Config**: `internal/config/config.go` (new `ServerConfig` struct and defaults).
- **Core Loop/Adapter integration**: The API server will act as a new M6/Adapter (like Discord/Telegram) that invokes `core.Brain`. It will need a mechanism to stream `Brain.Step` output back via HTTP SSE.
- **Providers Factory**: `internal/adapters/llm/provider.go`.
- **CLI**: `main.go` or the CLI router to support `agis serve`.
- **Doctor**: `internal/doctor/` to include server and expanded provider health checks.

## Architecture & Implementation Notes
1. **Brain Streaming and the API Server**: Currently, `core.Brain` uses a synchronous `sink Sink` callback. For the API server to implement `stream: true`, it will inject a request-scoped `Sink` that flushes SSE chunks to the `http.ResponseWriter`.
2. **Session Preservation**: The server will read `X-Session-ID` (or the OpenAI `user` field). If provided, it restores that conversation from the repository. If not, it creates an ephemeral or default session.
3. **Stateless vs Stateful Brains**: Since `Brain` holds `activeID` state, the server must either instantiate a new `Brain` per request OR the `Brain` must be refactored to accept context-bound session IDs rather than mutating a shared global instance. We will instantiate a new `Brain` execution path or properly isolate the `activeID` per request to avoid race conditions.

## Risks
1. **Concurrent Brain State Mutation**: If multiple API requests share one `core.Brain` instance and call `SetActiveConversation`, race conditions will occur.
   - *Mitigation*: The API Server will configure a fresh `Brain` runner per request, or we will introduce `StepWithSession(ctx, sessionID, input)` that doesn't mutate global Brain state.
2. **Anthropic/Cohere API Deviations**: They do not natively support the exact OpenAI `/v1/chat/completions` JSON schema.
   - *Mitigation*: We will build lightweight native adapters in `internal/adapters/llm/anthropic.go` and `cohere.go` instead of forcing them through a broken OpenAI compatibility layer.
3. **Loss of Client Context**: Standard OpenAI clients pass a full array of `messages`. If AGIS's `Brain.Step` only expects the *latest* user input (because it manages memory itself), sending the full array from the client might duplicate history.
   - *Mitigation*: The API server will extract only the *last* user message to pass to `Brain.Step`, treating AGIS as the stateful memory manager it is, unless explicitly configured to operate as a dumb passthrough.

## Rollback
- Revert the `internal/server/` directory addition.
- Revert CLI flag additions in `main.go`.
- Revert `ServerConfig` additions in `internal/config/config.go`.
- The provider factory changes are additive and defaults-based, but can be easily reverted if they cause fallback resolution issues.

## Success Criteria
- `agis serve` launches an HTTP server listening on a port.
- Standard OpenAI clients (e.g. `curl` or python `openai` library) can authenticate with a Bearer token and receive a response from AGIS.
- `stream: true` returns valid SSE chunks ending with `[DONE]`.
- All requested providers (DeepSeek, Gemini, etc.) can be configured via `config.yaml` using their familiar preset names, without needing manual base URLs.
- `agis doctor` tests the server configuration and provider endpoints without panicking.

## Proposal question round
*(Automatic mode: assumptions below will guide implementation unless rejected by a later phase)*
1. **Message History Handling**: Since OpenAI clients pass full message histories per request but AGIS manages its own memory (via RRF/vector search), we assume the server should extract only the **last** user message from the incoming request to feed into `Brain.Step`. Is this correct, or should the server replace AGIS memory entirely with the incoming payload for API calls?
2. **Brain Concurrency**: `core.Brain` currently has stateful `activeID`. We assume the API server will instantiate a fresh `Brain` per request (or pass the session directly without mutating a global Brain) to support concurrent API clients safely.
3. **Anthropic/Cohere Adapters**: We assume dedicated `anthropic.go` and `cohere.go` files will be built, as their native APIs do not perfectly match OpenAI schemas.