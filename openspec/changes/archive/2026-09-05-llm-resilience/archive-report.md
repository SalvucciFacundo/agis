# Archive Report: llm-resilience

## Change Overview
- **Name**: `llm-resilience`
- **Archived Date**: 2026-09-05
- **Status**: Completed & Archived
- **Mode**: Automatic (`auto`)
- **Artifact Store**: Hybrid (`openspec/` + Engram)
- **Delivery Strategy**: `auto-chain` (`stacked-to-main`)

## Summary of Accomplishments
1. **Configuration & Deep Secret Masking (`internal/config`)**:
   - Added `LLMConfig.APIKeys`, `LLMConfig.Fallbacks`, and `LLMFallbackConfig`.
   - Extended `MaskSecrets` to recursively redact all API keys across primary credential pools and fallback configurations (`[MASKED]`).
   - Extended `MemoryConfig` and `VisionConfig` with auxiliary `provider` and `model` overrides.
2. **Credential Pool & Reactive Key Rotation (`internal/adapters/llm`)**:
   - Implemented thread-safe `CredentialPool` with `sync.RWMutex`, key deduplication, and stampede-protected reactive `RotateKey` on HTTP 429 errors.
   - Integrated `CredentialPool` into `Client` for automatic retry with rotated keys.
3. **Error Classifier & Composite Fallback Provider (`internal/adapters/llm`)**:
   - Implemented `isTransientError` categorizing 429, 500, 502, 503, 504, and network timeouts as transient, while failing fast on 400, 401, 403, 404, and `context.Canceled`.
   - Implemented `FallbackProvider` wrapping ordered provider chains for `Chat` sequential failover and `Stream` pre-token failover with safe mid-stream error termination.
4. **Auxiliary Task Providers & Factory Helpers (`internal/adapters/llm`)**:
   - Implemented `NewResilientProvider` and `NewProviderForTask` to decouple memory curation, vision, and background tasks from the primary chat model.
5. **Diagnostics & Documentation (`internal/doctor`, `docs/`)**:
   - Updated `checkLLM` in `internal/doctor` with multi-endpoint probing, key pool size reporting, and graduated status semantics (`PASS`/`WARN`/`FAIL`).
   - Updated `docs/configuration.md`, `docs/cli.md`, and `README.md`.
   - Synced master specification to `openspec/specs/llm-resilience/spec.md`.

## Verification Results
- **Strict TDD Compliance**: 100% verified across all 4 work units.
- **Specification Requirements**: 12/12 requirements and scenarios satisfied (PASS).
- **Test Suite**: 24/24 Go packages passing with `go test -race -count=1 ./...` and clean `go vet ./...` (zero races, zero leaks, zero vet warnings).

## Final State Facts
- Packages added/modified: `internal/config`, `internal/adapters/llm`, `internal/doctor`, `cmd/agis`.
- Artifacts archived to: `openspec/changes/archive/2026-09-05-llm-resilience/`
- Master spec at: `openspec/specs/llm-resilience/spec.md`
