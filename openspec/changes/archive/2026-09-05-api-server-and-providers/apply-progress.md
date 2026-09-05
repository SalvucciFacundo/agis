# Apply Progress: OpenAI-Compatible REST API Server & Expanded LLM Providers (api-server-and-providers)

## Status Summary

- **Change**: `api-server-and-providers`
- **Current Slice**: Batch 2 (PR 3: Chat Completions & SSE Streaming Engine + PR 4: CLI Subcommand, Doctor Diagnostics & Documentation)
- **Delivery Strategy**: `auto-chain` (Stacked PRs to main)
- **Strict TDD**: Active and enforced across all work units
- **All Implementation Tasks**: Completed (PR 1, PR 2, PR 3, PR 4)

---

## Completed Work Units

### PR 1: Configuration, Secret Masking & Provider Presets Catalog
- **Completed Tasks**:
  - `internal/config/config.go`: Added `ServerConfig` struct with defaults (`Host: "127.0.0.1"`, `Port: 8080`, `ReadTimeout: 30s`, `WriteTimeout: 120s`, `CORSOrigins: ["*"]`).
  - `internal/config/mask.go`: Added masking for `Server.APIKey` (`sk-***`) and `MaskConfig` helper.
  - `internal/adapters/llm/presets.go`: Implemented canonical presets catalog for OpenAI, Ollama, Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, Together, Cohere with `ResolveBaseURL`.
  - `internal/adapters/llm/anthropic.go`: Implemented native Anthropic Messages API adapter supporting system messages extraction, chat turns, tool definitions, tool use, and SSE `content_block_delta` streaming.
  - `internal/adapters/llm/provider.go` & `openai.go`: Updated provider constructors to seamlessly resolve presets and instantiate Anthropic / OpenAI / Ollama adapters.

### PR 2: Server Foundations & Middleware
- **Completed Tasks**:
  - `internal/server/types.go`: OpenAI-compatible schemas for chat completions, chunks, models listing, health responses, and error envelopes.
  - `internal/server/auth.go`: Constant-time Bearer token authentication middleware using `crypto/subtle.ConstantTimeCompare`, supporting open mode when API key is empty and bypassing health checks.
  - `internal/server/cors.go`: CORS middleware handling preflight `OPTIONS` requests (returning HTTP 204 with headers) and matching allowed origins for GET/POST.
  - `internal/server/server.go`: HTTP server foundations with `http.ServeMux`, lifecycle management (`Start`, `Shutdown`), health checks (`GET /healthz`, `GET /v1/health`), and models listing (`GET /v1/models`).

### PR 3: Chat Completions & SSE Streaming Engine
- **Completed Tasks**:
  - `internal/server/chat.go`: Implemented `POST /v1/chat/completions` supporting non-streaming JSON responses and streaming SSE chunks (`text/event-stream`, `data: [DONE]`).
  - `internal/server/chat.go`: Implemented session resolution via `X-Session-ID` header and `user` payload parameter, extracting latest user turns and multimodal attachments.
  - `internal/core/brain.go`: Added thread-safe `StepWithSession` and `StepWithSessionAndAttachments` supporting request-scoped sinks and session binding.
  - `internal/server/chat_test.go`: Thorough unit tests validating non-streaming completions, SSE chunk generation, context cancellation, error responses, and goroutine leak freedom with `goleak.VerifyNone`.

### PR 4: CLI Subcommand, Doctor Diagnostics & Documentation
- **Completed Tasks**:
  - `cmd/agis/serve.go` & `cmd/agis/main.go`: Implemented `agis serve` (with `agis api` alias) CLI subcommand with flag overrides (`-host`, `-port`, `-api-key`, `-cors`, `-profile`, `-config`) and signal-based graceful shutdown.
  - `internal/doctor/server.go` & `internal/doctor/doctor.go`: Implemented `checkServer` diagnostic probe verifying port availability and emitting security warnings on public interfaces with empty API keys.
  - `docs/cli.md`, `docs/configuration.md`, `README.md`: Updated comprehensive documentation covering `agis serve`, `server:` config block, and provider presets catalog.
  - Full project test suite and race verification (`go test -race -count=1 ./...` and `go vet ./...`) passing with 100% success.

---

## TDD Cycle Evidence

| Work Unit / Feature | RED Phase Test | GREEN Phase Impl | REFACTOR & Race Detection |
|---------------------|----------------|------------------|---------------------------|
| Server Config & Masking | `internal/config/config_test.go:TestLoad_ServerDefaultsAndExplicit`, `internal/config/mask_test.go:TestMaskSecrets_ServerAPIKey` | `internal/config/config.go`, `internal/config/mask.go` | `go test -race -count=1 ./internal/config/...` (PASS) |
| Provider Presets Catalog | `internal/adapters/llm/presets_test.go:TestProviderPresets_Resolution` | `internal/adapters/llm/presets.go`, `internal/adapters/llm/provider.go` | `go test -race -count=1 ./internal/adapters/llm/...` (PASS) |
| Anthropic Messages API Adapter | `internal/adapters/llm/anthropic_test.go:TestAnthropic_Chat`, `TestAnthropic_Stream`, `TestAnthropic_Stream_ToolUse` | `internal/adapters/llm/anthropic.go` | `go test -race -count=1 ./internal/adapters/llm/...` (PASS) |
| Auth Middleware | `internal/server/auth_test.go:TestAuthMiddleware` | `internal/server/auth.go` | `go test -race -count=1 ./internal/server/...` (PASS) |
| CORS Middleware | `internal/server/cors_test.go:TestCORSMiddleware` | `internal/server/cors.go` | `go test -race -count=1 ./internal/server/...` (PASS) |
| Server Foundations & Models / Health | `internal/server/server_test.go:TestServer_HealthEndpoints`, `TestServer_ModelsEndpoint`, `TestServer_Lifecycle` | `internal/server/server.go`, `internal/server/types.go` | `go test -race -count=1 ./internal/server/...` (PASS) |
| Chat Completions (Non-Streaming & SSE) | `internal/server/chat_test.go:TestChatCompletions_NonStreaming`, `TestChatCompletions_StreamingSSE`, `TestChatCompletions_ErrorHandling`, `TestChatCompletions_ContextCancellation`, `TestChatCompletions_MultimodalContent` | `internal/server/chat.go`, `internal/core/brain.go` | `go test -race -count=1 ./internal/server/...` (PASS) |
| CLI `agis serve` / `agis api` | `cmd/agis/serve_test.go:TestServeCLI_Help`, `TestServeCLI_RunWithContextCancel` | `cmd/agis/serve.go`, `cmd/agis/main.go` | `go test -race -count=1 ./cmd/agis/...` (PASS) |
| Doctor Server Diagnostic Probe | `internal/doctor/server_test.go:TestDoctor_CheckServer_Localhost`, `TestDoctor_CheckServer_PublicWithoutAuthWarning`, `TestDoctor_CheckServer_PortInUse` | `internal/doctor/server.go`, `internal/doctor/doctor.go` | `go test -race -count=1 ./internal/doctor/...` (PASS) |

---

## Files Changed

- `cmd/agis/main.go`
- `cmd/agis/serve.go`
- `cmd/agis/serve_test.go`
- `docs/cli.md`
- `docs/configuration.md`
- `internal/adapters/llm/anthropic.go`
- `internal/adapters/llm/anthropic_test.go`
- `internal/adapters/llm/openai.go`
- `internal/adapters/llm/presets.go`
- `internal/adapters/llm/presets_test.go`
- `internal/adapters/llm/provider.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/mask.go`
- `internal/config/mask_test.go`
- `internal/core/brain.go`
- `internal/doctor/doctor.go`
- `internal/doctor/server.go`
- `internal/doctor/server_test.go`
- `internal/server/auth.go`
- `internal/server/auth_test.go`
- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `internal/server/cors.go`
- `internal/server/cors_test.go`
- `internal/server/server.go`
- `internal/server/server_test.go`
- `internal/server/types.go`
- `openspec/changes/api-server-and-providers/apply-progress.md`
- `openspec/changes/api-server-and-providers/tasks.md`
- `README.md`

---

## Verification Commands & Output

```bash
$ go test -race -count=1 ./...
ok  	github.com/SalvucciFacundo/agis/cmd/agis	4.587s
ok  	github.com/SalvucciFacundo/agis/internal/adapters/llm	1.151s
ok  	github.com/SalvucciFacundo/agis/internal/adapters/tui	1.496s
ok  	github.com/SalvucciFacundo/agis/internal/config	1.086s
ok  	github.com/SalvucciFacundo/agis/internal/core	1.021s
ok  	github.com/SalvucciFacundo/agis/internal/cron	1.681s
ok  	github.com/SalvucciFacundo/agis/internal/doctor	1.186s
ok  	github.com/SalvucciFacundo/agis/internal/gateway	1.291s
ok  	github.com/SalvucciFacundo/agis/internal/mcp	1.106s
ok  	github.com/SalvucciFacundo/agis/internal/mcp/transport	1.217s
ok  	github.com/SalvucciFacundo/agis/internal/memory	5.520s
ok  	github.com/SalvucciFacundo/agis/internal/persona	1.007s
ok  	github.com/SalvucciFacundo/agis/internal/plugins	1.011s
ok  	github.com/SalvucciFacundo/agis/internal/policy	1.345s
ok  	github.com/SalvucciFacundo/agis/internal/scan	1.005s
ok  	github.com/SalvucciFacundo/agis/internal/server	1.180s
ok  	github.com/SalvucciFacundo/agis/internal/session	1.714s
ok  	github.com/SalvucciFacundo/agis/internal/setup	1.124s
ok  	github.com/SalvucciFacundo/agis/internal/skills	1.008s
ok  	github.com/SalvucciFacundo/agis/internal/subagents	1.241s
ok  	github.com/SalvucciFacundo/agis/internal/tools	1.171s
ok  	github.com/SalvucciFacundo/agis/internal/tools/web/fetch	1.328s
ok  	github.com/SalvucciFacundo/agis/internal/tools/web/search	1.118s
ok  	github.com/SalvucciFacundo/agis/internal/updater	1.025s
ok  	github.com/SalvucciFacundo/agis/internal/version	1.008s
ok  	github.com/SalvucciFacundo/agis/internal/webhook	1.118s

$ go vet ./...
(zero issues)
```

---

## Remaining Work Units

All implementation tasks across PR 1, PR 2, PR 3, and PR 4 are complete.
The change is ready for verification (`sdd-verify`).
