# Tasks: OpenAI-Compatible REST API Server & Expanded LLM Providers (api-server-and-providers)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1,450 - 1,850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Configuration & LLM Presets/Anthropic) → PR 2 (Server Foundations & Middleware) → PR 3 (Chat Completions & SSE Streaming) → PR 4 (CLI, Doctor Probes & Docs) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

---

## Work Units

### PR 1: Configuration, Secret Masking & Provider Presets Catalog
- [x] Implement and verify `internal/config/config.go` extensions for `ServerConfig` with sensible defaults (`Host: "127.0.0.1"`, `Port: 8080`, timeouts, CORS). <!-- sdd-owner: implementation -->
- [x] Implement and verify secret masking in `internal/config/mask.go` for `Server.APIKey` (`sk-***`). <!-- sdd-owner: implementation -->
- [x] Implement and verify provider presets catalog in `internal/adapters/llm/presets.go` supporting major LLM providers (Anthropic, DeepSeek, Groq, Gemini, OpenRouter, Mistral, xAI, Together, Cohere) with automatic BaseURL resolution. <!-- sdd-owner: implementation -->
- [x] Implement and verify native Anthropic Messages API adapter in `internal/adapters/llm/anthropic.go` handling system prompts, chat turns, tool usage, and SSE `content_block_delta` streaming. <!-- sdd-owner: implementation -->
- [x] Implement unit tests for configuration loading, masking, preset resolution, and Anthropic adapter translation in `internal/config/` and `internal/adapters/llm/`. <!-- sdd-owner: implementation -->

### PR 2: Server Foundations & Middleware
- [x] Implement and verify HTTP server foundation in `internal/server/server.go` using `http.ServeMux` and robust timeout/lifecycle options. <!-- sdd-owner: implementation -->
- [x] Implement and verify constant-time Bearer token authentication middleware in `internal/server/auth.go` using `crypto/subtle.ConstantTimeCompare` and OpenAI-compatible 401 error payloads. <!-- sdd-owner: implementation -->
- [x] Implement and verify CORS middleware in `internal/server/cors.go` supporting preflight `OPTIONS` requests and configurable allowed origins. <!-- sdd-owner: implementation -->
- [x] Implement and verify `GET /v1/models` and health check endpoints (`GET /healthz`, `GET /v1/health`) in `internal/server/` with structured status responses. <!-- sdd-owner: implementation -->
- [x] Implement unit tests for server routing, auth middleware, CORS, models listing, and health checks using `httptest`. <!-- sdd-owner: implementation -->

### PR 3: Chat Completions & SSE Streaming Engine
- [x] Implement request parsing, session resolution (`X-Session-ID` / `user`), and non-streaming `POST /v1/chat/completions` handler in `internal/server/chat.go`. <!-- sdd-owner: implementation -->
- [x] Implement request-scoped SSE streaming engine (`http.Flusher`, chunk delta formatting, and terminal `[DONE]` marker) in `internal/server/chat.go`. <!-- sdd-owner: implementation -->
- [x] Implement context cancellation handling to clean up active `Brain.Step` executions when client connections drop mid-stream. <!-- sdd-owner: implementation -->
- [x] Implement thorough unit and race tests (`go test -race ./internal/server/...`) validating concurrent chat completions, SSE formatting, and context cleanup without goroutine leaks. <!-- sdd-owner: implementation -->

### PR 4: CLI Subcommand, Doctor Diagnostics & Documentation
- [x] Implement `agis serve` (with `agis api` alias) subcommand in `cmd/agis/serve.go` with flag overrides and signal-based graceful shutdown. <!-- sdd-owner: implementation -->
- [x] Implement server diagnostic probe in `internal/doctor/server.go` checking port bindability and public interface security warnings. <!-- sdd-owner: implementation -->
- [x] Implement expanded LLM provider reachability probes in `internal/doctor/doctor.go`. <!-- sdd-owner: implementation -->
- [x] Write user and developer documentation in `docs/cli.md` and `docs/configuration.md`, and update `README.md`. <!-- sdd-owner: implementation -->
- [x] Run full integration test suite, verify strict TDD compliance, and perform final review preparation. <!-- sdd-owner: implementation -->

---

## Key Learnings

1. Breaking down architectural changes into stacked PRs (Configuration/Presets -> Middleware/Foundations -> Streaming Chat Engine -> CLI/Doctor) ensures clean review boundaries and keeps change lines per PR well-managed.
2. Constant-time comparison using `crypto/subtle.ConstantTimeCompare` is critical for secure Bearer token authentication in public-facing API servers.
3. Request-scoped SSE streaming requires robust `http.Flusher` assertions and immediate context cancellation propagation to prevent hanging goroutines during client disconnects.
