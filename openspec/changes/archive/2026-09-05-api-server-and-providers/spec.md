# Specification: OpenAI-Compatible REST API Server & Expanded LLM Providers (api-server-and-providers)

## Purpose

Define the functional requirements, architectural boundaries, API schemas, concurrency guarantees, authentication policies, and diagnostic probes for the REST API server and expanded LLM provider ecosystem in AGIS (`agis`). This change introduces:
1. **OpenAI-Compatible REST API Server (`internal/server/`)**: An HTTP server exposing standard `/v1/chat/completions` (streaming SSE and non-streaming), `/v1/models`, and `/v1/health` endpoints that map third-party client requests to the AGIS cognitive loop (`core.Brain`).
2. **Session Preservation & Multi-Turn State**: Header and body session mappings (`X-Session-ID` and `user`) connecting external API clients to persistent AGIS conversation histories.
3. **Security & Authentication Middleware**: Constant-time Bearer token verification and configurable Cross-Origin Resource Sharing (CORS).
4. **Expanded LLM Provider Catalog (`internal/adapters/llm/`)**: Built-in presets for Anthropic, Google Gemini, DeepSeek, Groq, Mistral, xAI, Together AI, Cohere, and OpenRouter with automatic base URL and header injection.
5. **CLI Subcommand (`agis serve` / `agis api`)**: Interactive and daemonized execution with signal-based graceful shutdown.
6. **Observability & Health Probes (`internal/doctor/`)**: Diagnostic checks validating server configuration, port binding, and provider reachability.

---

## 1. HTTP API Server (`internal/server/`)

### Requirement SRV-HTTP-001: Server Lifecycle & Graceful Shutdown
The system MUST provide an HTTP Server instance configurable via options (`WithAddr`, `WithAPIKey`, `WithCORS`, `WithBrainRunner`, `WithLogger`, `WithTimeouts`).
- The server MUST listen on the host and port specified in configuration or CLI flags (default `127.0.0.1:8080`).
- The server MUST support graceful shutdown via `Shutdown(ctx context.Context)`: closing listeners, rejecting new incoming connections with HTTP 503, and allowing active in-flight chat completion requests up to a configurable timeout (default: 10s) to drain before terminating.

#### Scenario: Server starts and binds to configured address
- GIVEN a `ServerConfig` specifying `Host: "127.0.0.1"` and `Port: 8080`
- WHEN `server.Start()` is invoked
- THEN the HTTP listener successfully binds to `127.0.0.1:8080` and begins accepting connections

#### Scenario: Graceful shutdown drains active connections
- GIVEN an active in-flight request on `POST /v1/chat/completions`
- WHEN `server.Shutdown(ctx)` is triggered via SIGINT or context cancellation
- THEN the listener stops accepting new connections
- AND the in-flight request is allowed to complete within the shutdown grace period

---

### Requirement SRV-AUTH-001: Bearer Token Authentication Middleware
The server MUST provide authentication middleware securing all `/v1/*` endpoints (except `/healthz` and `/v1/health`).
- If `ServerConfig.APIKey` is non-empty, every incoming request MUST include an `Authorization: Bearer <token>` header matching the configured key.
- Token comparison MUST use `crypto/subtle.ConstantTimeCompare` to prevent timing attacks.
- If the token is missing, malformed, or does not match, the middleware MUST immediately reject the request with HTTP 401 Unauthorized and an OpenAI-compatible JSON error envelope:
  ```json
  {
    "error": {
      "message": "Incorrect API key provided or missing Bearer token.",
      "type": "invalid_request_error",
      "param": null,
      "code": "invalid_api_key"
    }
  }
  ```
- If `ServerConfig.APIKey` is empty, authentication MUST be bypassed (open mode), but a warning MUST be logged if bound to a non-loopback address.

#### Scenario: Request with valid Bearer token succeeds
- GIVEN `server.api_key` configured as `"secret-token-123"`
- WHEN a request is sent with `Authorization: Bearer secret-token-123`
- THEN the authentication middleware passes and the downstream handler is executed

#### Scenario: Request with invalid or missing Bearer token returns 401
- GIVEN `server.api_key` configured as `"secret-token-123"`
- WHEN a request is sent without an `Authorization` header or with `Authorization: Bearer wrong-token`
- THEN the server returns HTTP status 401 Unauthorized with standard OpenAI error JSON

#### Scenario: Empty API key allows unauthenticated local access
- GIVEN `server.api_key` is empty (`""`)
- WHEN a request is sent without any `Authorization` header
- THEN the request is accepted and processed

---

### Requirement SRV-CORS-001: CORS Middleware
The server MUST provide a Cross-Origin Resource Sharing (CORS) middleware configured by `ServerConfig.CORSOrigins`.
- If `CORSOrigins` contains entries (e.g. `["*"]` or `["http://localhost:3000"]`), the server MUST respond to `OPTIONS` preflight requests with HTTP 204 No Content and headers:
  - `Access-Control-Allow-Origin: <origin>`
  - `Access-Control-Allow-Methods: GET, POST, OPTIONS`
  - `Access-Control-Allow-Headers: Authorization, Content-Type, X-Session-ID, Accept`
  - `Access-Control-Max-Age: 86400`
- For regular GET/POST requests, matching origins MUST receive the appropriate `Access-Control-Allow-Origin` response header.

#### Scenario: Preflight OPTIONS request receives CORS headers
- GIVEN `CORSOrigins` set to `["*"]`
- WHEN an `OPTIONS` request is sent with `Origin: http://localhost:3000`
- THEN the server responds with HTTP 204 and `Access-Control-Allow-Origin: *`

---

### Requirement SRV-CHAT-001: Non-Streaming Chat Completions (`POST /v1/chat/completions`)
The server MUST accept OpenAI-standard chat completion requests and return an OpenAI-standard response JSON object.
- **Request Body JSON Schema**:
  ```json
  {
    "model": "string (optional, defaults to active AGIS model)",
    "messages": [
      {
        "role": "user | assistant | system",
        "content": "string or array of content parts"
      }
    ],
    "stream": false,
    "temperature": 0.7,
    "user": "string (optional session identifier)"
  }
  ```
- **Execution & Session Resolution**:
  - The server MUST resolve the conversation ID from (in order of priority): `X-Session-ID` header, `user` body parameter, or generate a deterministic or ephemeral session ID.
  - The server MUST extract the newest user message content (and multimodal image attachments if present) from `messages`.
  - The server MUST invoke the AGIS cognitive loop (`Brain.Step`) bound to the resolved session ID.
- **Response JSON Schema (`chat.completion`)**:
  ```json
  {
    "id": "chatcmpl-<uuid>",
    "object": "chat.completion",
    "created": 1740000000,
    "model": "<model_name>",
    "choices": [
      {
        "index": 0,
        "message": {
          "role": "assistant",
          "content": "<assistant reply text>"
        },
        "finish_reason": "stop"
      }
    ],
    "usage": {
      "prompt_tokens": 0,
      "completion_tokens": 0,
      "total_tokens": 0
    }
  }
  ```

#### Scenario: Valid non-streaming chat request
- GIVEN a valid JSON payload `{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello AGIS"}]}`
- WHEN `POST /v1/chat/completions` is called with `stream: false`
- THEN the server processes the prompt through `Brain.Step` and returns HTTP 200 with an OpenAI-compatible completion JSON

#### Scenario: Malformed request payload returns 400
- GIVEN an invalid JSON payload or missing `messages`
- WHEN `POST /v1/chat/completions` is called
- THEN the server returns HTTP 400 Bad Request with an OpenAI error envelope (`"type": "invalid_request_error"`)

---

### Requirement SRV-CHAT-002: Streaming Chat Completions (`POST /v1/chat/completions` with `stream: true`)
When `stream: true` is requested, the server MUST stream tokens back to the client using Server-Sent Events (`text/event-stream`).
- The response MUST set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, and `X-Accel-Buffering: no`.
- As tokens arrive from the provider during `Brain.Step`, the request-scoped `Sink` MUST immediately format each chunk as SSE and flush to the `http.ResponseWriter`:
  ```
  data: {"id":"chatcmpl-<uuid>","object":"chat.completion.chunk","created":1740000000,"model":"<model>","choices":[{"index":0,"delta":{"content":"<token>"},"finish_reason":null}]}\n\n
  ```
- Upon completion of the turn, the server MUST emit a terminal finish chunk with `finish_reason: "stop"` followed by:
  ```
  data: [DONE]\n\n
  ```
- If the client disconnects before completion (`r.Context().Done()`), the server MUST abort `Brain.Step` execution cleanly without leaking goroutines.

#### Scenario: Streaming chat returns valid SSE chunks and terminates with [DONE]
- GIVEN a request with `stream: true` and input `"Tell me a short joke"`
- WHEN `POST /v1/chat/completions` is called
- THEN the HTTP response headers are set to `text/event-stream`
- AND chunks containing token deltas are flushed sequentially
- AND the stream concludes with `data: [DONE]\n\n`

#### Scenario: Client disconnection cancels Brain execution
- GIVEN an active streaming chat request
- WHEN the client closes the TCP connection mid-stream
- THEN the request context is cancelled and `Brain.Step` halts immediately

---

### Requirement SRV-MODELS-001: Models Listing Endpoint (`GET /v1/models`)
The server MUST expose `GET /v1/models` returning available models in OpenAI list format.
- Response Schema:
  ```json
  {
    "object": "list",
    "data": [
      {
        "id": "<model_id>",
        "object": "model",
        "created": 1740000000,
        "owned_by": "<provider_name>"
      }
    ]
  }
  ```
- The list MUST include the primary active model and all configured fallback and auxiliary task models.

#### Scenario: Query models returns active model and provider
- GIVEN AGIS configured with primary model `"llama3.2"` on provider `"ollama"`
- WHEN `GET /v1/models` is called
- THEN the response status is 200 and data contains `{"id": "llama3.2", "object": "model", "owned_by": "ollama"}`

---

### Requirement SRV-HEALTH-001: Health Check Endpoints (`GET /healthz` & `GET /v1/health`)
The server MUST expose unauthenticated health check endpoints at `/healthz` and `/v1/health`.
- Response format:
  ```json
  {
    "status": "ok",
    "version": "0.1.0",
    "profile": "default",
    "active_provider": "ollama",
    "active_model": "llama3.2"
  }
  ```
- The endpoint MUST return HTTP 200 OK.

#### Scenario: Health endpoint returns system status
- GIVEN the API server is running
- WHEN `GET /healthz` or `GET /v1/health` is requested
- THEN the server returns HTTP 200 with JSON payload containing status, version, and active profile

---

## 2. Expanded LLM Providers Ecosystem (`internal/adapters/llm/`)

### Requirement LLM-PRESET-001: Built-in Provider Catalog & Defaults
The LLM adapter layer MUST maintain a built-in catalog of major LLM providers with canonical base URLs and default protocols in `internal/adapters/llm/presets.go`.
- Supported preset names (resolved case-insensitively):
  1. `ollama`: `http://localhost:11434` (Ollama native / OpenAI-compatible endpoint)
  2. `openai`: `https://api.openai.com/v1` (OpenAI standard API)
  3. `openrouter`: `https://openrouter.ai/api/v1` (OpenRouter unified API)
  4. `gemini`: `https://generativelanguage.googleapis.com/v1beta/openai/` (Google Gemini OpenAI compatibility endpoint)
  5. `deepseek`: `https://api.deepseek.com/v1` (DeepSeek API)
  6. `groq`: `https://api.groq.com/openai/v1` (Groq Fast Inference API)
  7. `mistral`: `https://api.mistral.ai/v1` (Mistral AI API)
  8. `xai`: `https://api.x.ai/v1` (xAI Grok API)
  9. `together`: `https://api.together.xyz/v1` (Together AI API)
  10. `cohere`: `https://api.cohere.com/v2` (Cohere API)
  11. `anthropic`: `https://api.anthropic.com` (Anthropic Messages API)
  12. `custom`: Generic OpenAI-compatible endpoint (requires explicit `base_url`).
- If `cfg.BaseURL` is omitted or empty, `NewProvider` MUST automatically supply the canonical default Base URL for the specified provider.

#### Scenario: Provider created with preset name resolves default Base URL
- GIVEN `cfg := config.LLMConfig{Provider: "deepseek", Model: "deepseek-chat", APIKey: "sk-xxx"}` with empty `BaseURL`
- WHEN `NewProvider(cfg)` is called
- THEN the adapter is initialized using `https://api.deepseek.com/v1` as the base URL

#### Scenario: Explicit BaseURL overrides provider preset default
- GIVEN `cfg := config.LLMConfig{Provider: "ollama", BaseURL: "http://192.168.1.50:11434"}`
- WHEN `NewProvider(cfg)` is called
- THEN the adapter uses the custom provided base URL `http://192.168.1.50:11434`

---

### Requirement LLM-ANTHROPIC-001: Anthropic Messages Adapter (`internal/adapters/llm/anthropic.go`)
The system MUST provide an Anthropic adapter implementing the `core.Provider` interface (`Chat`, `Stream`, `Models`).
- The adapter MUST communicate with Anthropic's `/v1/messages` endpoint.
- Requests MUST include headers:
  - `x-api-key: <api_key>`
  - `anthropic-version: 2023-06-01`
  - `content-type: application/json`
- The adapter MUST translate `core.ChatRequest` to Anthropic's message format:
  - Extract system messages into the top-level `system` field.
  - Map user/assistant turns into the `messages` array.
  - Handle tool definitions (`tools` schema) and parse `tool_use` blocks from responses.
- `Stream` MUST process Anthropic SSE stream events (`content_block_delta`, `message_delta`, `message_stop`) and emit normalized `core.StreamEvent` tokens on the output channel.

#### Scenario: Anthropic Chat returns assistant message
- GIVEN an Anthropic provider configured with API key and model `claude-3-5-sonnet-20241022`
- WHEN `Chat` is invoked with a user message
- THEN the adapter formats a `/v1/messages` request, sends `x-api-key` header, and parses the response into `core.ChatResponse`

#### Scenario: Anthropic Stream translates SSE content deltas
- GIVEN an Anthropic streaming request
- WHEN Anthropic server sends `content_block_delta` SSE chunks
- THEN the stream channel yields `StreamEvent{Text: delta_text}` and closes cleanly on `message_stop`

---

### Requirement LLM-COHERE-001: Cohere Adapter (`internal/adapters/llm/cohere.go`)
The system MUST provide a Cohere adapter or OpenAI-compatible shim implementing `core.Provider` for Cohere models (e.g. `command-r-plus`).
- Supports standard `Chat` and `Stream` operations, sending `Authorization: Bearer <api_key>` and normalizing response structures to `core.ChatResponse` and `core.StreamEvent`.

#### Scenario: Cohere provider executes chat turn
- GIVEN a Cohere configuration with provider `cohere` and model `command-r-plus`
- WHEN `Chat` is called
- THEN the request succeeds and returns normalized `ChatResponse`

---

## 3. Configuration & Secret Masking (`internal/config/`)

### Requirement CFG-SRV-001: Server Configuration Structure & Defaults
The `Config` struct MUST include a `Server ServerConfig` block in `internal/config/config.go`.
- Struct definition:
  ```go
  type ServerConfig struct {
      Enabled      bool          `yaml:"enabled"`
      Host         string        `yaml:"host"`
      Port         int           `yaml:"port"`
      APIKey       string        `yaml:"api_key"`
      CORSOrigins  []string      `yaml:"cors_origins"`
      ReadTimeout  time.Duration `yaml:"read_timeout"`
      WriteTimeout time.Duration `yaml:"write_timeout"`
  }
  ```
- **Defaults**:
  - `Host`: `"127.0.0.1"`
  - `Port`: `8080`
  - `ReadTimeout`: `30s`
  - `WriteTimeout`: `120s`
  - `CORSOrigins`: `["*"]`

#### Scenario: Default configuration populates server defaults
- GIVEN an empty config file
- WHEN `config.Load()` is called
- THEN `cfg.Server.Host` is `"127.0.0.1"`, `cfg.Server.Port` is `8080`, and `cfg.Server.ReadTimeout` is `30s`

---

### Requirement CFG-MASK-001: Server API Key Masking
`config.MaskConfig(cfg)` MUST mask `cfg.Server.APIKey` using the standard secret masking rules (`sk-***` or `***`).
- The raw API key MUST never be exposed in plaintext in logs, doctor reports, or serialization dumps.

#### Scenario: MaskConfig redacts server API key
- GIVEN `cfg.Server.APIKey = "sk-srv-secret123456"`
- WHEN `MaskConfig(cfg)` is executed
- THEN the resulting struct contains `Server.APIKey = "sk-***"`

---

## 4. CLI Subcommand (`cmd/agis/serve.go` & `cmd/agis/main.go`)

### Requirement CLI-SRV-001: Subcommand `agis serve` / `agis api`
The AGIS CLI MUST support the `serve` subcommand (with `api` as an alias) to start the HTTP API server.
- Supported CLI flags:
  - `-host string`: HTTP host to bind (overrides `server.host`).
  - `-port int`: HTTP port to bind (overrides `server.port`).
  - `-api-key string`: Bearer token for authentication (overrides `server.api_key`).
  - `-cors string`: Comma-separated allowed CORS origins.
  - `-profile string`: Active configuration profile name.
  - `-config string`: Path to explicit YAML configuration file.
- Signal Handling: The CLI runner MUST trap `SIGINT` (Ctrl+C) and `SIGTERM` and invoke graceful shutdown on the server.

#### Scenario: agis serve runs and logs startup info
- GIVEN `agis serve -port 9090`
- WHEN the command executes
- THEN the server initializes the repository, LLM provider, and brain, and logs `"listening on 127.0.0.1:9090"`

#### Scenario: Interrupt signal stops server gracefully
- GIVEN `agis serve` running in foreground
- WHEN SIGINT is sent
- THEN the server prints `"shutting down gracefully..."`, finishes active requests, and exits with code 0

---

## 5. Diagnostic Probes (`internal/doctor/`)

### Requirement DOC-SRV-001: Server Configuration & Port Diagnostic Probe
`internal/doctor` MUST include a diagnostic probe checking the API server setup.
- Checks performed:
  1. Port availability: Verifies that the configured `Host:Port` is bindable or currently owned by AGIS.
  2. Security check: If `Host` is `0.0.0.0` (public interface) and `APIKey` is empty, emits `StatusWarn` warning that the server is exposed without authentication.
  3. Status result: Returns `StatusPass` if host/port configuration is valid and secure.

#### Scenario: Doctor passes on standard localhost config
- GIVEN `server.host: "127.0.0.1"` and `server.port: 8080`
- WHEN `doctor.Run()` executes
- THEN the `"server"` check reports `StatusPass`

#### Scenario: Doctor warns on public binding without API key
- GIVEN `server.host: "0.0.0.0"` and `server.api_key: ""`
- WHEN `doctor.Run()` executes
- THEN the `"server"` check reports `StatusWarn` with a security warning

---

### Requirement DOC-PROV-001: Expanded Provider Health Probes
`internal/doctor` MUST include checks validating the reachability and credentials for all configured primary, fallback, and auxiliary LLM providers from the expanded catalog.
- For each configured provider:
  - Validates API key presence (if required by provider).
  - Performs a lightweight reachability ping / models check against the resolved Base URL.
  - Reports `StatusPass`, `StatusWarn`, or `StatusFail` per provider.

#### Scenario: Doctor probes configured DeepSeek provider
- GIVEN a configured primary provider `deepseek` with API key
- WHEN `doctor.Run()` executes
- THEN the probe validates API key presence and endpoint reachability, returning `StatusPass` if reachable

---

## Key Learnings

1. Preserving conversational context in stateless REST API environments requires mapping transport metadata (headers and user identifiers) directly to durable session storage before invoking the agent loop.
2. Supporting streaming SSE and non-streaming responses concurrently within the same core loop requires a request-scoped token sink and decoupled context cancellation handlers.
3. Exposing HTTP endpoints on external network interfaces necessitates constant-time token validation and static diagnostic warnings to prevent accidental unauthenticated access.
