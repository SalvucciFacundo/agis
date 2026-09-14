# Tasks: Text-to-Speech (TTS) Outbound Voice Synthesis

## Review Workload Forecast
- Estimated changed lines: ~450 additions, ~25 deletions
- 400-line budget risk: Low/Medium
- Chained PRs recommended: No (Single cohesive architectural unit)
- Delivery strategy: Single PR stacked to main
- TDD mode: Strict TDD (RED -> GREEN -> REFACTOR)

---

## Work Units

### Work Unit 1: Domain Port & Core Definitions (`internal/core`)
- [x] Define `Synthesizer` interface in `internal/core/port_synthesizer.go`.
- [x] Add unit tests in `internal/core/synthesizer_test.go`.
- [x] Verify test pass: `go test -race ./internal/core/...`.

### Work Unit 2: Audio Synthesis Adapters (`internal/adapters/audio`)
- [x] Implement `OpenAITTS` in `internal/adapters/audio/openai.go`.
- [x] Implement `ElevenLabsTTS` in `internal/adapters/audio/elevenlabs.go`.
- [x] Implement factory `NewSynthesizer` in `internal/adapters/audio/factory.go`.
- [x] Write unit tests with mock HTTP servers in `internal/adapters/audio/audio_test.go`.
- [x] Verify test pass: `go test -race ./internal/adapters/audio/...`.

### Work Unit 3: Configuration Schema & Defaults (`internal/config`)
- [x] Add `TTSConfig` struct to `internal/config/config.go`.
- [x] Add `TTS` field to `MultimodalConfig` and root `Config`.
- [x] Implement default population and validation logic.
- [x] Add unit tests in `internal/config/config_test.go`.
- [x] Verify test pass: `go test -race ./internal/config/...`.

### Work Unit 4: Gateway Voice Sending & Multiplexer Integration (`internal/gateway`, `cmd/agis`)
- [x] Define `VoiceSender` interface in `internal/gateway/adapter.go`.
- [x] Implement `SendVoice` on `TelegramAdapter` in `internal/gateway/telegram.go`.
- [x] Implement `SendVoice` on `WhatsAppAdapter` in `internal/gateway/whatsapp.go`.
- [x] Integrate `Synthesizer` into `Multiplexer` with automatic voice replies on audio inbound and graceful fallback to text.
- [x] Wire `Synthesizer` in `cmd/agis/gateway.go`.
- [x] Write unit and integration tests in `internal/gateway/...`.
- [x] Verify test pass: `go test -race ./internal/gateway/...`.

### Work Unit 5: Verification, Documentation & Roadmap
- [x] Update `docs/development/hermes-parity-roadmap.md` (mark Fase 11 as ✅ DONE).
- [x] Update `docs/gateway.md` and `docs/configuration.md` with TTS options.
- [x] Run full regression suite: `go test -race ./...` and `go vet ./...`.
- [x] Commit with conventional commits and push.
