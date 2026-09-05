# Specification: LLM Resilience, Credential Pooling, and Auxiliary Overrides (llm-resilience)

## Purpose

Define the functional requirements, architectural boundaries, failure classification, concurrency guarantees, security policies, and diagnostic probes for the LLM resilience subsystem in AGIS (`agis`). This subsystem provides:
1. **Fallback Provider Chain**: Automatic failover across primary and secondary LLM providers for transient network and API errors, including pre-token streaming failover.
2. **Credential Pooling & Reactive Rotation**: Thread-safe multi-key rotation per provider to mitigate HTTP 429 rate limits before escalating to next providers.
3. **Auxiliary Task Model Overrides**: Independent provider/model specifications for memory curation, vision, audio, and embeddings, decoupling background tasks from main conversation models.
4. **Configuration & Secret Masking**: Safe serialization and logging of multi-key credentials and fallback trees.
5. **Doctor Health Probes**: Comprehensive diagnostic validation of all primary endpoints, key pools, and fallback chains.

---

## 1. Fallback Provider (`internal/adapters/llm/fallback.go`)

### Requirement RES-FALL-001: Composite Provider Interface Implementation
The system MUST provide a `FallbackProvider` struct implementing the `core.Provider` interface (`Chat`, `Stream`, `Models`).
- `FallbackProvider` MUST encapsulate an ordered chain of `core.Provider` instances: `[PrimaryProvider, Fallback1, Fallback2, ...]`.
- If no fallback providers are configured, `FallbackProvider` MUST delegate directly to the primary provider without overhead or behavioral divergence.
- When `Models()` is called, `FallbackProvider` MUST return the available models from the primary provider, augmented with model identifiers from fallback providers.

#### Scenario: Fallback provider initialization with chain
- GIVEN a primary provider (OpenAI / `gpt-4o`) and two fallback providers (OpenRouter / `claude-3-5-sonnet`, Ollama / `llama3.2`)
- WHEN `NewFallbackProvider(primary, fallbacks...)` is instantiated
- THEN the composite provider maintains an ordered list of 3 providers with primary at index 0

#### Scenario: Fallback provider with no secondary fallbacks
- GIVEN a configuration with only a primary provider and empty fallbacks
- WHEN `Chat` or `Stream` is executed
- THEN requests execute against the primary provider and return results without additional retry/failover attempts

---

### Requirement RES-FALL-002: Error Classification & Transient Detection
The resilience subsystem MUST accurately classify errors as transient (eligible for retry and failover) or non-transient (fatal, fast-fail).
- The system MUST define `isTransientError(err error) bool`.
- **Transient Errors (Failover Permitted)**:
  - HTTP 429 (Too Many Requests / Rate Limit Exceeded).
  - HTTP 500 (Internal Server Error).
  - HTTP 502 (Bad Gateway).
  - HTTP 503 (Service Unavailable).
  - HTTP 504 (Gateway Timeout).
  - Network timeouts (implementing `net.Error` with `Timeout() == true`).
  - Connection refused, connection reset by peer, or EOF before HTTP response headers received.
- **Non-Transient Errors (Fast-Fail, No Failover)**:
  - HTTP 400 (Bad Request / invalid schema or parameters).
  - HTTP 401 (Unauthorized / Invalid API Key) — *unless* alternative unexhausted keys remain in that provider's `CredentialPool`.
  - HTTP 403 (Forbidden / Access Denied).
  - HTTP 404 (Not Found / Model does not exist).
  - Context cancellation (`context.Canceled`).
  - Explicit client-side context deadline expiration (`context.DeadlineExceeded` initiated by parent context).

#### Scenario: Transient HTTP 503 triggers failover
- GIVEN a primary provider returning HTTP 503 Service Unavailable
- WHEN `Chat` is invoked on `FallbackProvider`
- THEN `isTransientError` classifies the error as transient
- AND `FallbackProvider` advances to the next provider in the chain and retries the request

#### Scenario: Non-transient HTTP 400 fails immediately
- GIVEN a primary provider returning HTTP 400 Bad Request
- WHEN `Chat` is invoked on `FallbackProvider`
- THEN `isTransientError` classifies the error as non-transient
- AND `FallbackProvider` immediately returns the HTTP 400 error without attempting secondary providers

#### Scenario: Context cancellation aborts immediately
- GIVEN an active request whose context is cancelled (`context.Canceled`)
- WHEN `FallbackProvider` receives the cancellation error
- THEN execution terminates immediately without attempting any fallback providers

---

### Requirement RES-FALL-003: Non-Streaming (`Chat`) Failover Execution
The `Chat` method of `FallbackProvider` MUST attempt execution across the ordered chain of providers upon encountering transient errors.
- `FallbackProvider` MUST iterate from primary (index 0) through fallbacks (indices `1..N`).
- If a provider succeeds, `Chat` MUST immediately return the `core.ChatResponse` and `nil` error.
- If a provider encounters a transient error:
  - If the provider uses a `CredentialPool` and has remaining untried keys, it MUST rotate keys and retry within the same provider first.
  - If keys are exhausted or the provider error persists, `FallbackProvider` MUST proceed to the next provider in the chain.
- If all providers in the chain fail, `Chat` MUST return a wrapped error detailing all attempted providers and the terminal failure.

#### Scenario: Primary succeeds on first attempt
- GIVEN a functioning primary provider
- WHEN `Chat` is invoked
- THEN the response is returned from the primary provider and fallback providers are never called

#### Scenario: Primary fails with HTTP 500, secondary succeeds
- GIVEN a primary provider that fails with HTTP 500
- AND a secondary fallback provider that is operational
- WHEN `Chat` is invoked
- THEN the primary failure is logged, secondary provider is invoked, and secondary response is returned successfully

#### Scenario: All providers fail
- GIVEN primary and all fallback providers failing with HTTP 503
- WHEN `Chat` is invoked
- THEN `Chat` returns an error indicating failure across all chain members (`"all LLM providers failed: [primary: status 503, fallback-1: status 503]"`)

---

### Requirement RES-FALL-004: Streaming (`Stream`) Pre-Token & Mid-Stream Failover Semantics
The `Stream` method of `FallbackProvider` MUST enforce strict streaming invariants based on token emission state.
- **Pre-Token Failure**:
  - If an error occurs before the first `core.StreamEvent` containing non-empty `Text` or `ToolCall` is emitted to the output channel, `FallbackProvider` MUST cancel the failed stream and failover to the next provider in the chain.
  - The client MUST receive a seamless stream from the winning fallback provider without noticing prior failed attempts.
- **Mid-Stream Failure (After >= 1 token emitted)**:
  - If a transient network disconnect, stream EOF, or provider error occurs after one or more tokens/tool-calls have already been flushed to the output channel:
    - `FallbackProvider` MUST NOT attempt failover to a new provider (to prevent duplicated prefixes or incoherent stitched completions).
    - `FallbackProvider` MUST emit a terminal `core.StreamEvent{Err: err}`.
    - `FallbackProvider` MUST close the channel cleanly.

#### Scenario: Pre-token failure triggers seamless stream failover
- GIVEN a primary provider that fails immediately with HTTP 502 when opening stream
- AND a fallback provider that is healthy
- WHEN `Stream` is called
- THEN no events are emitted from the primary
- AND `FallbackProvider` opens a stream with the secondary provider
- AND the client receives the secondary provider's stream events followed by normal channel closure

#### Scenario: Mid-stream disconnect terminates with error event
- GIVEN a primary provider that emits 3 text tokens (`"Hello"`, `" world"`, `" from"`)
- AND subsequently encounters a read timeout / network break
- WHEN the stream failure occurs
- THEN the output channel receives `core.StreamEvent{Err: <timeout error>}`
- AND the channel is immediately closed without invoking fallback providers

---

## 2. Credential Pool & Key Rotation (`internal/adapters/llm/pool.go`)

### Requirement RES-POOL-001: Thread-Safe Credential Pool
The system MUST provide a `CredentialPool` struct to manage multiple API credentials for a single provider.
- `CredentialPool` MUST support construction via `NewCredentialPool(primaryKey string, keys []string) *CredentialPool`.
- `CredentialPool` MUST deduplicate keys while preserving order, ensuring `primaryKey` is the first element if not already present.
- If both `primaryKey` and `keys` are empty, `CredentialPool` MUST behave safely, returning empty string `""` without panicking.
- All operations on `CredentialPool` MUST be safe for concurrent access across multiple goroutines using synchronization primitives (`sync.RWMutex` or atomic operations).

#### Scenario: Deduplication and order preservation
- GIVEN `primaryKey = "key-A"` and `keys = ["key-B", "key-A", "key-C", "key-B"]`
- WHEN `NewCredentialPool(primaryKey, keys)` is invoked
- THEN the pool contains exactly `["key-A", "key-B", "key-C"]` in that order

#### Scenario: Single key backward compatibility
- GIVEN `primaryKey = "key-only"` and `keys = nil`
- WHEN `NewCredentialPool(primaryKey, keys)` is invoked
- THEN `pool.Len()` returns `1` and `pool.CurrentKey()` returns `"key-only"`

---

### Requirement RES-POOL-002: Reactive Key Rotation
`CredentialPool` MUST provide a `RotateKey(failedKey string) (string, bool)` method for reactive rate-limit recovery.
- When an API request encounters HTTP 429 or 401, the adapter MUST invoke `RotateKey(failedKey)`.
- **Idempotency & Race Protection**:
  - If multiple concurrent requests fail simultaneously with the same `failedKey`, `RotateKey` MUST advance the active key index only once.
  - If `failedKey` does not match the currently active key (meaning another goroutine has already rotated past it), `RotateKey` MUST return the current key without advancing again.
- **Exhaustion Detection**:
  - `RotateKey` MUST return `(nextKey, true)` if a new/different key is available.
  - If the pool contains only 1 key or all keys in the pool have been cycled through for the current request cycle, `RotateKey` MUST signal exhaustion by returning `(currentKey, false)`.

#### Scenario: Concurrent rotation on 429 only advances once
- GIVEN a pool with keys `["k1", "k2", "k3"]` and active key `"k1"`
- WHEN 5 concurrent goroutines all receive HTTP 429 and call `RotateKey("k1")`
- THEN the active key advances to `"k2"` exactly once
- AND subsequent calls with `"k1"` receive `"k2"` without advancing to `"k3"`

#### Scenario: Single-key pool exhaustion
- GIVEN a pool with only `["k1"]`
- WHEN `RotateKey("k1")` is called
- THEN it returns `("k1", false)` indicating no alternate key exists

---

### Requirement RES-POOL-003: HTTP Client Authorization Header Injection
The HTTP client adapter in `internal/adapters/llm/client.go` MUST integrate with `CredentialPool`.
- `Client` MUST retrieve the active key from its `CredentialPool` on every request.
- When injecting headers, if `pool.CurrentKey()` is non-empty, the request header MUST be set to `Authorization: Bearer <currentKey>`.
- If an HTTP 429 is returned and `pool.RotateKey` returns `(nextKey, true)`, `Client` MUST retry the request with the new key before returning an error to the caller.

#### Scenario: Automatic retry on 429 with rotated key
- GIVEN an OpenAI adapter configured with keys `["key-1", "key-2"]`
- WHEN a request with `"key-1"` receives HTTP 429
- THEN the client automatically rotates to `"key-2"`
- AND retries the HTTP request with `Authorization: Bearer key-2`
- AND returns the successful response to the caller

---

## 3. Auxiliary Task Model Overrides

### Requirement RES-AUX-001: Independent Model & Provider Overrides
The configuration schema and provider resolution system MUST allow auxiliary subsystems to specify dedicated models and providers independent of the primary LLM configuration.
- **Memory Curation (`MemoryConfig`)**:
  - MUST support optional `provider` (`string`) and `model` (`string`).
  - When specified, memory curation tasks (`memory.Curator`) MUST instantiate a dedicated provider targeting that model/provider.
  - When omitted, memory curation MUST default to the primary `llm.provider` and `llm.model`.
- **Vision Subsystem (`VisionConfig`)**:
  - MUST support optional `provider` (`string`) and `model` (`string`).
  - When omitted, vision tasks MUST fallback to primary `llm.provider` / `llm.model`.
- **Audio Transcription (`AudioConfig`)**:
  - MUST support `provider` (`string`) and `model` (`string`).
- **Embeddings Subsystem (`EmbeddingsConfig`)**:
  - MUST support `provider` (`string`) and `model` (`string`).

#### Scenario: Memory curation uses lightweight local model while chat uses remote model
- GIVEN `llm.provider: "openai"`, `llm.model: "gpt-4o"`
- AND `memory.provider: "ollama"`, `memory.model: "llama3.2:1b"`
- WHEN `memory.Curator` is constructed
- THEN the curator uses the Ollama provider with `"llama3.2:1b"` while chat interactions continue using OpenAI `"gpt-4o"`

#### Scenario: Auxiliary config inherits primary defaults when unspecified
- GIVEN `llm.provider: "openai"`, `llm.model: "gpt-4o"`
- AND `memory.provider: ""` and `memory.model: ""`
- WHEN `memory.Curator` is constructed
- THEN the curator uses the OpenAI provider with `"gpt-4o"`

---

### Requirement RES-AUX-002: Factory Helper for Task Provider Resolution
The system MUST provide a factory function (e.g. `NewProviderForTask(baseCfg config.LLMConfig, taskProvider, taskModel string) core.Provider`) in `internal/adapters/llm/provider.go`.
- If `taskProvider` is empty, it MUST inherit `baseCfg.Provider`.
- If `taskModel` is empty, it MUST inherit `baseCfg.Model`.
- The factory MUST return a valid `core.Provider` instance initialized with the resolved configuration.

#### Scenario: Helper resolves partial overrides
- GIVEN `baseCfg` with `Provider: "openai"`, `Model: "gpt-4o"`, `APIKey: "sk-..."`
- WHEN `NewProviderForTask(baseCfg, "", "gpt-4o-mini")` is called
- THEN it returns an OpenAI provider configured for model `"gpt-4o-mini"` with the base API key

---

## 4. Configuration & Secret Masking (`internal/config`)

### Requirement RES-CONF-001: Configuration Schema for Fallbacks & Credential Pools
The configuration loader in `internal/config/config.go` MUST support fallback provider chains and multi-key credential pools.
- `LLMConfig` MUST include:
  ```go
  type LLMConfig struct {
      Provider  string              `yaml:"provider"`
      Model     string              `yaml:"model"`
      APIKey    string              `yaml:"api_key"`
      APIKeys   []string            `yaml:"api_keys"`
      BaseURL   string              `yaml:"base_url,omitempty"`
      Fallbacks []LLMFallbackConfig `yaml:"fallbacks"`
  }
  
  type LLMFallbackConfig struct {
      Provider string   `yaml:"provider"`
      Model    string   `yaml:"model"`
      APIKey   string   `yaml:"api_key"`
      APIKeys  []string `yaml:"api_keys"`
      BaseURL  string   `yaml:"base_url,omitempty"`
  }
  ```
- YAML parsing MUST seamlessly parse both single `api_key` and multiple `api_keys` for primary and all fallback entries.

#### Scenario: YAML with fallbacks and multiple keys parsed correctly
- GIVEN a `config.yaml` containing:
  ```yaml
  llm:
    provider: openai
    model: gpt-4o
    api_keys: ["sk-1", "sk-2"]
    fallbacks:
      - provider: openrouter
        model: anthropic/claude-3.5-sonnet
        api_key: sk-or-1
      - provider: ollama
        model: llama3.2
  ```
- WHEN `config.Load()` is executed
- THEN `cfg.LLM.APIKeys` has 2 items
- AND `cfg.LLM.Fallbacks` has 2 fallback configurations with their respective providers, models, and keys

---

### Requirement RES-CONF-002: Complete Secret Masking for Multi-Key Configurations
The secret masking function `config.MaskSecrets(cfg *Config) *Config` in `internal/config/mask.go` MUST mask all sensitive API keys across primary pools and fallback configurations.
- `cfg.LLM.APIKey` MUST be replaced with `"[MASKED]"` if non-empty.
- Every element in `cfg.LLM.APIKeys` MUST be replaced with `"[MASKED]"`.
- For every fallback in `cfg.LLM.Fallbacks`:
  - `fallback.APIKey` MUST be replaced with `"[MASKED]"` if non-empty.
  - Every element in `fallback.APIKeys` MUST be replaced with `"[MASKED]"`.
- The original `Config` struct MUST remain unaltered (masking performed on a deep copy).

#### Scenario: MaskSecrets masks all fallback and pooled keys
- GIVEN a `Config` with `LLM.APIKeys = ["secret-1", "secret-2"]` and a fallback with `APIKey = "secret-3"`
- WHEN `MaskSecrets(cfg)` is called
- THEN the returned config contains `LLM.APIKeys = ["[MASKED]", "[MASKED]"]` and `Fallbacks[0].APIKey = "[MASKED]"`
- AND the input `cfg` retains `"secret-1"`, `"secret-2"`, `"secret-3"`

---

## 5. Doctor Diagnostic Probe (`internal/doctor`)

### Requirement RES-DOC-001: Comprehensive LLM Diagnostics for Chains & Pools
The `checkLLM` diagnostic check in `internal/doctor/doctor.go` MUST probe primary connectivity, key pool status, and all configured fallback providers.
- **Primary Check**:
  - Verify primary provider reachability and model availability.
  - Report the number of configured keys in the primary credential pool.
- **Fallback Checks**:
  - For each configured fallback in `cfg.LLM.Fallbacks`, probe endpoint reachability and report status in `CheckResult.Details`.
- **Status Classification**:
  - `PASS`: Primary provider is healthy and reachable.
  - `WARN`: Primary provider is unhealthy/unreachable, BUT at least one configured fallback provider is healthy and reachable.
  - `FAIL`: Both primary provider AND all configured fallback providers are unreachable/failing.

#### Scenario: Primary healthy produces PASS
- GIVEN primary OpenAI provider is reachable
- AND 1 fallback provider is configured
- WHEN `doctor.Run(ctx)` executes
- THEN the `"llm"` check returns `StatusPass` with details listing primary success and fallback readiness

#### Scenario: Primary down but fallback operational produces WARN
- GIVEN primary provider endpoint is offline (connection refused)
- AND fallback Ollama provider is reachable and responsive
- WHEN `doctor.Run(ctx)` executes
- THEN the `"llm"` check returns `StatusWarn`
- AND the message notes `"Primary provider failed, but fallback provider(s) are operational"`
- AND details list the specific failure of the primary and success of the fallback

#### Scenario: All providers down produces FAIL
- GIVEN primary provider and all fallback providers unreachable
- WHEN `doctor.Run(ctx)` executes
- THEN the `"llm"` check returns `StatusFail`
- AND details enumerate the connection failures for every provider in the chain

---

## Summary of Acceptance Criteria

| Requirement ID | Domain | Key Invariant |
|---|---|---|
| `RES-FALL-001` | Fallback Provider | Composite `core.Provider` maintaining ordered chain `[Primary, Fallback1, ...]`. |
| `RES-FALL-002` | Error Classification | Transient (429, 500, 502, 503, 504, net timeouts) failover; Non-transient (400, cancel) fail fast. |
| `RES-FALL-003` | Chat Failover | Sequential failover across chain; exhausts key pool before next provider; detailed chain error if all fail. |
| `RES-FALL-004` | Stream Failover | Pre-token failure triggers seamless next-provider failover; Mid-stream failure terminates cleanly with error event without re-streaming. |
| `RES-POOL-001` | Credential Pool | Thread-safe key slice, deduplication, order preservation, empty pool safe. |
| `RES-POOL-002` | Reactive Rotation | Idempotent race-protected `RotateKey(failedKey)` advances once per distinct failure; detects cycle exhaustion. |
| `RES-POOL-003` | Client Injection | Injects active key as `Bearer <key>`; retries 429 automatically with rotated key. |
| `RES-AUX-001` | Auxiliary Overrides | `MemoryConfig`, `VisionConfig`, `AudioConfig`, `EmbeddingsConfig` support independent provider/model overrides. |
| `RES-AUX-002` | Factory Helpers | `NewProviderForTask` cleanly resolves task overrides against base defaults. |
| `RES-CONF-001` | Config Schema | `LLMFallbackConfig`, `LLMConfig.Fallbacks`, `LLMConfig.APIKeys` parsed from YAML. |
| `RES-CONF-002` | Secret Masking | Deep copy obfuscates all primary and fallback keys to `[MASKED]`. |
| `RES-DOC-001` | Doctor Diagnostics | Probes primary + all fallbacks; PASS if primary ok, WARN if primary down but fallback ok, FAIL if all down. |
