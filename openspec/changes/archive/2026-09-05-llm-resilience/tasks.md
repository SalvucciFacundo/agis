## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~650 lines |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

---

## Work Units

### PR 1: Configuration, Secret Masking & Credential Pool
- [x] **Task 1.1 (RED)**: Write unit tests in `internal/config/config_test.go` and `internal/config/mask_test.go` for YAML parsing of `api_keys` and `fallbacks`, and deep secret masking across primary pools and fallback configurations. <!-- sdd-owner: implementation -->
- [x] **Task 1.2 (GREEN)**: Implement `LLMConfig`, `LLMFallbackConfig`, and schema loading extensions in `internal/config/config.go`. Implement deep secret masking in `internal/config/mask.go`. <!-- sdd-owner: implementation -->
- [x] **Task 1.3 (RED)**: Write unit tests in `internal/adapters/llm/pool_test.go` covering `NewCredentialPool` deduplication, order preservation, empty states, concurrent rate-limit rotation, and cycle exhaustion. <!-- sdd-owner: implementation -->
- [x] **Task 1.4 (GREEN)**: Implement thread-safe `CredentialPool` with `sync.RWMutex`, reactive `RotateKey`, and race protection in `internal/adapters/llm/pool.go`. Run tests with `-race`. <!-- sdd-owner: implementation -->

### PR 2: Error Classifier & Fallback Provider Core
- [x] **Task 2.1 (RED)**: Write table-driven unit tests in `internal/adapters/llm/errors_test.go` for `isTransientError` covering HTTP status codes (400, 401, 429, 500-504), network timeouts, and context cancellations. <!-- sdd-owner: implementation -->
- [x] **Task 2.2 (GREEN)**: Implement `isTransientError` in `internal/adapters/llm/errors.go`. <!-- sdd-owner: implementation -->
- [x] **Task 2.3 (RED)**: Write unit tests in `internal/adapters/llm/fallback_test.go` covering `FallbackProvider` initialization, sequential `Chat` failover, credential rotation integration, and aggregate error reporting. <!-- sdd-owner: implementation -->
- [x] **Task 2.4 (GREEN)**: Implement `FallbackProvider` struct and `Chat` failover execution in `internal/adapters/llm/fallback.go`. <!-- sdd-owner: implementation -->
- [x] **Task 2.5 (RED)**: Write unit tests in `internal/adapters/llm/fallback_stream_test.go` covering pre-token streaming failover (seamless transition), mid-stream termination (no failover after token emission), and goroutine cleanup (`goleak`). <!-- sdd-owner: implementation -->
- [x] **Task 2.6 (GREEN)**: Implement `Stream` failover semantics with pre-token/mid-stream state machines and proper cancellation in `internal/adapters/llm/fallback.go`. <!-- sdd-owner: implementation -->

### PR 3: Client Key Rotation Integration & Auxiliary Overrides
- [x] **Task 3.1 (RED)**: Write unit tests in `internal/adapters/llm/client_test.go` verifying HTTP Authorization header injection from `CredentialPool` and automatic 429 retry with rotated keys using `httptest.Server`. <!-- sdd-owner: implementation -->
- [x] **Task 3.2 (GREEN)**: Integrate `CredentialPool` and reactive `RotateKey` retry logic into `Client` in `internal/adapters/llm/client.go`. <!-- sdd-owner: implementation -->
- [x] **Task 3.3 (RED)**: Write unit tests in `internal/adapters/llm/provider_test.go` for `NewProviderForTask` and auxiliary configuration defaults (`MemoryConfig`, `VisionConfig`, `AudioConfig`, `EmbeddingsConfig`). <!-- sdd-owner: implementation -->
- [x] **Task 3.4 (GREEN)**: Implement `NewProviderForTask` and auxiliary provider resolution in `internal/adapters/llm/provider.go`. Wire auxiliary subsystems to use dedicated overrides or inherit primary defaults. <!-- sdd-owner: implementation -->

### PR 4: Doctor Health Diagnostics, Main Wiring & Documentation
- [x] **Task 4.1 (RED)**: Write unit tests in `internal/doctor/doctor_test.go` for `checkLLM` verifying `PASS` when primary is healthy, `WARN` when primary fails but fallback is operational, and `FAIL` when all providers fail. <!-- sdd-owner: implementation -->
- [x] **Task 4.2 (GREEN)**: Update `checkLLM` in `internal/doctor/doctor.go` to probe primary, key pool status, and all configured fallbacks. Wire main app provider initialization in `cmd/agis/main.go` to construct `FallbackProvider` chains. <!-- sdd-owner: implementation -->
- [x] **Task 4.3 (REFACTOR & VERIFICATION)**: Run full test suite across all packages with race detector (`go test -race ./...`) and goroutine leak detection. Update `docs/configuration.md`, `docs/cli.md`, and `README.md` to document LLM fallbacks, credential pools, and auxiliary overrides. <!-- sdd-owner: implementation -->
