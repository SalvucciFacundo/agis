# Proposal: LLM Resilience, Credential Pooling, and Auxiliary Overrides

## 1. Intent
Implement a multi-layer LLM resilience subsystem for AGIS to handle transient API failures, rate limiting (HTTP 429), and backend downtime gracefully. Introduce API credential pools with reactive round-robin rotation, seamless provider failover (including pre-token streaming support), and independent model configurations for auxiliary tasks (vision, audio, memory, embeddings).

## 2. Business Problem & Current State Gap
Currently, AGIS relies on a single LLM provider and model for core chat tasks. If the provider experiences an outage, network timeout, or hits a rate limit, the request fails entirely. Auxiliary tasks (like memory curation) inherit the primary chat model, which may be suboptimal (e.g., using a high-latency reasoning model for a simple summarization task). Furthermore, high-throughput scenarios risk exhausting single API keys, leading to poor user experience.

## 3. Product Outcome
- **Resilience**: Transient errors automatically failover to secondary providers (e.g., OpenAI -> OpenRouter -> Ollama).
- **API Key Pools**: Multiple keys per provider distribute load and mitigate HTTP 429 rate limits through reactive rotation.
- **Task Optimization**: Vision, audio, embeddings, and memory curation can each specify dedicated lightweight or specialized models/providers independent of the primary chat model.
- **Observability**: The `agis doctor` diagnostic command validates not just the primary provider, but all configured fallback endpoints and credential pools.

## 4. Scope & Affected Areas

### In Scope
- **Configuration (`internal/config/config.go`)**:
  - Add `api_keys` and `fallbacks` to `LLMConfig`.
  - Add `Provider` to `VisionConfig`.
  - Add `Provider` and `Model` to `MemoryConfig`.
  - Keep `EmbeddingsConfig` and `AudioConfig` updated if needed (they already have Provider/Model fields).
- **Core Interface (`internal/core/provider.go`)**:
  - Existing `core.Provider` interface remains stable. Introduce a composite `FallbackProvider` that implements it.
- **LLM Adapters (`internal/adapters/llm`)**:
  - Implement `FallbackProvider` coordinating the primary provider and a chain of fallback providers.
  - Implement a thread-safe `CredentialPool` for each adapter to manage `api_keys`, rotating upon encountering HTTP 429 or 50x errors.
  - Handle streaming failover: if `Stream()` fails before the first token, reroute to the next provider cleanly. If it fails mid-stream, emit a clear error event and log without attempting to stitch a new response.
- **Diagnostics (`internal/doctor/doctor.go`)**:
  - Extend `checkLLM` to iterate over primary and all `fallbacks`.
  - Validate connectivity and credential validity for multiple `api_keys`.
- **Security**:
  - Ensure all API keys in credential pools and fallback configurations are properly masked in logs (`[MASKED]`).

### Out of Scope
- Load balancing across multiple endpoints of the same provider (outside of credential pooling).
- Cross-provider conversational memory merging (AGIS handles memory at a higher layer).
- Automatic retry backoffs inside the stream mid-generation.

## 5. Business Rules & Implications
- **Failover Trigger**: Only trigger failover on transient/recoverable errors: HTTP 429, 500, 502, 503, 504, connection timeouts, or network unreachable. HTTP 400 (Bad Request) or 401 (Unauthorized - if all keys fail) should fast-fail.
- **Credential Rotation**: Key rotation must be thread-safe. A 429 response rotates the active key index and retries the *same* provider before failing over to the *next* provider.
- **Streaming Invariants**: If the first token has already been flushed to the client, a failover cannot rewrite history. We must abort the stream cleanly with an error indicator.
- **Security Logs**: No cleartext API keys may be logged during fallback transitions or credential rotations.

## 6. Edge Cases
- **Empty Key Pools**: Gracefully handle configurations where `api_key` is provided but `api_keys` is empty, ensuring full backward compatibility.
- **All Providers Fail**: If the primary and all fallbacks exhaust their keys and retry limits, return a wrapped error indicating the entire chain failed and what providers were attempted.
- **Mid-Stream Death**: Network disconnection while streaming tokens must be detected and propagated to the client.

## 7. Rollback Plan
- The `FallbackProvider` serves as a wrapper. If fallback issues occur, it can be disabled by simply omitting the `fallbacks` array from `config.yaml`. The existing `LLMConfig.APIKey` and `Provider` fields will function precisely as they do now.

## 8. Success Criteria
- [ ] `FallbackProvider` successfully routes to a secondary provider on a simulated HTTP 500.
- [ ] `CredentialPool` successfully rotates keys on HTTP 429 and succeeds on the retry.
- [ ] Auxiliary tasks (e.g., Memory curation) can successfully utilize an independent local model while the primary chat uses a remote model.
- [ ] `agis doctor` outputs a comprehensive matrix of checks for primary and fallback LLM targets.
- [ ] No API keys appear in application logs or standard output.
