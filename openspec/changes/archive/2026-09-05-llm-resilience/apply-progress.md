# SDD Apply Progress: llm-resilience

## Summary
- **Change**: `llm-resilience`
- **Batch**: Batch 2 (PR 3: Client Key Rotation Integration & Auxiliary Overrides + PR 4: Doctor Health Diagnostics, Main Wiring & Documentation) — Final Implementation Slice
- **Status**: All Tasks Complete (PR 1, PR 2, PR 3, PR 4)
- **Strict TDD Mode**: Active (`go test -race -count=1 ./...`)

---

## TDD Cycle Evidence

| Task / Feature | RED Phase Test | GREEN Phase Implementation | TRIANGULATE / REFACTOR | Race / Leak Evidence |
|---|---|---|---|---|
| **1.1 & 1.2 Config & Masking** | `TestLoad_LLMFallbacksAndAPIKeys`, `TestMaskSecrets_LLMFallbacksAndAPIKeys` in `internal/config/` | Added `LLMConfig.APIKeys`, `LLMConfig.Fallbacks`, `LLMFallbackConfig` to `internal/config/config.go`; updated `maskFields` in `internal/config/mask.go` | Preserved backwards compatibility with single `api_key` and non-secret fields | Passed `go test -race -count=1 ./internal/config/...` |
| **1.3 & 1.4 CredentialPool** | `TestNewCredentialPool_DeduplicationAndOrder`, `TestCredentialPool_RotateKey`, `TestCredentialPool_ConcurrentRotation_StampedePrevention` in `internal/adapters/llm/pool_test.go` | Implemented `CredentialPool` with `sync.RWMutex`, `CurrentKey()`, `RotateKey()`, `Len()` in `internal/adapters/llm/pool.go` | Added stampede prevention returning active key on stale rotation without advancing index | Passed `go test -race -count=1 ./internal/adapters/llm/...` |
| **2.1 & 2.2 Error Classifier** | `TestIsTransientError` table-driven test in `internal/adapters/llm/errors_test.go` (covering 400, 401, 403, 404, 429, 500-504, net.Error timeouts, EOF, context cancellations) | Implemented `isTransientError(err error) bool` in `internal/adapters/llm/errors.go` | Evaluated explicit context cancellations and fatal status codes first before transient markers | Passed `go test -race -count=1 ./internal/adapters/llm/...` |
| **2.3 & 2.4 FallbackProvider Chat** | `TestFallbackProvider_PrimarySuccess`, `TestFallbackProvider_PrimaryTransientFail_SecondarySucceeds`, `TestFallbackProvider_NonTransientFail_FastFails`, `TestFallbackProvider_ContextCanceled_FastFails`, `TestFallbackProvider_AllProvidersFail`, `TestFallbackProvider_Models`, `TestFallbackProvider_SingleProviderDirect` in `internal/adapters/llm/fallback_test.go` | Implemented `FallbackProvider` struct, constructor, `Models()`, and `Chat()` failover loop in `internal/adapters/llm/fallback.go` | Deduplicated combined models from all chain providers | Passed `go test -race -count=1 ./internal/adapters/llm/...` |
| **2.5 & 2.6 FallbackProvider Stream** | `TestFallbackProvider_Stream_PreTokenFailover`, `TestFallbackProvider_Stream_PreTokenChannelErrorFailover`, `TestFallbackProvider_Stream_MidStreamErrorTerminates`, `TestFallbackProvider_Stream_ContextCancellation`, `TestFallbackProvider_Stream_AllProvidersFailPreToken` in `internal/adapters/llm/fallback_stream_test.go` | Implemented `Stream()` in `internal/adapters/llm/fallback.go` with pre-token failover and mid-stream termination | Verified goroutine leak freedom on stream cancellations and mid-stream terminations | Passed `goleak.VerifyNone` and `go test -race -count=1 ./internal/adapters/llm/...` |
| **3.1 & 3.2 Client CredentialPool & 429 Retry** | `TestClient_AuthorizationHeaderInjection`, `TestClient_Chat_RateLimit_AutoRotationAndRetry`, `TestClient_Stream_RateLimit_AutoRotationAndRetry` in `internal/adapters/llm/client_test.go` | Integrated `CredentialPool` and reactive `RotateKey` retry loop in `Client.doChat` (`internal/adapters/llm/client.go`), updated `NewOpenAI` and `NewOllama` | Preserved full backward compatibility for `NewClient(baseURL, apiKey)` while supporting `NewClientWithPool` | Passed `go test -race -count=1 ./internal/adapters/llm/...` |
| **3.3 & 3.4 Auxiliary Overrides & Factories** | `TestNewProviderForTask`, `TestNewResilientProvider` in `internal/adapters/llm/provider_test.go` | Implemented `NewProviderForTask` and `NewResilientProvider` in `internal/adapters/llm/provider.go`; added `Provider` & `Model` overrides to `MemoryConfig` and `VisionConfig` | Verified fallback chains construct `FallbackProvider` and task overrides cleanly resolve against base configs | Passed `go test -race -count=1 ./internal/adapters/llm/...` |
| **4.1 & 4.2 Doctor Diagnostics & App Wiring** | `TestDoctor_LLM_ResilienceAndFallbacks` in `internal/doctor/doctor_test.go` (asserting `PASS` for primary healthy, `WARN` for primary fail + fallback ok, `FAIL` for all down) | Updated `checkLLM` and added `probeSingleLLM` in `internal/doctor/doctor.go`; wired `NewResilientProvider` and `NewProviderForTask` in `cmd/agis/main.go`, `cmd/agis/gateway.go`, `cmd/agis/webhook.go`, `cmd/agis/cron.go` | Verified pool key reporting and detailed fallback reachability diagnostics | Passed `go test -race -count=1 ./internal/doctor/...` and `cmd/agis` |
| **4.3 Verification & Documentation** | Ran `go test -race -count=1 ./...` and `go vet ./...` across all 24 project packages | Updated `docs/configuration.md`, `docs/cli.md`, and `README.md` | All project tests pass with race detector and goroutine leak assertions | Passed `go test -race -count=1 ./...` |

---

## Files Changed

- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/config/mask.go`
- `internal/config/mask_test.go`
- `internal/adapters/llm/pool.go`
- `internal/adapters/llm/pool_test.go`
- `internal/adapters/llm/errors.go`
- `internal/adapters/llm/errors_test.go`
- `internal/adapters/llm/fallback.go`
- `internal/adapters/llm/fallback_test.go`
- `internal/adapters/llm/fallback_stream_test.go`
- `internal/adapters/llm/client.go`
- `internal/adapters/llm/client_test.go`
- `internal/adapters/llm/openai.go`
- `internal/adapters/llm/ollama.go`
- `internal/adapters/llm/provider.go`
- `internal/adapters/llm/provider_test.go`
- `internal/doctor/doctor.go`
- `internal/doctor/doctor_test.go`
- `cmd/agis/main.go`
- `cmd/agis/gateway.go`
- `cmd/agis/webhook.go`
- `cmd/agis/cron.go`
- `docs/configuration.md`
- `docs/cli.md`
- `README.md`
- `openspec/changes/llm-resilience/tasks.md`
- `openspec/changes/llm-resilience/apply-progress.md`

---

## Test Commands Run
- `go test -race -count=1 ./internal/config/...` -> PASS
- `go test -race -count=1 ./internal/adapters/llm/...` -> PASS
- `go test -race -count=1 ./internal/doctor/...` -> PASS
- `go test -race -count=1 ./...` -> PASS across all 24 packages in AGIS repository
- `go vet ./...` -> PASS (clean, zero warnings)

---

## Deviations from Design
None. Implementation strictly fulfills ADRs D1 through D7.

---

## Remaining Tasks
None. All tasks across PR 1, PR 2, PR 3, and PR 4 are complete and checked off in `tasks.md`.

---

## Workload / PR Boundary
- **PR 1**: Configuration, Secret Masking & Credential Pool (`internal/config/`, `internal/adapters/llm/pool.go`)
- **PR 2**: Error Classifier & Fallback Provider Core (`internal/adapters/llm/errors.go`, `internal/adapters/llm/fallback.go`)
- **PR 3**: Client Key Rotation Integration & Auxiliary Overrides (`internal/adapters/llm/client.go`, `internal/adapters/llm/provider.go`)
- **PR 4**: Doctor Health Diagnostics, Main Wiring & Documentation (`internal/doctor/doctor.go`, `cmd/agis/`, `docs/`, `README.md`)
