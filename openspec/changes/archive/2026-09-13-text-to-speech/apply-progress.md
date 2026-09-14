# Apply Progress: Text-to-Speech (TTS) Outbound Voice Synthesis

## Status: COMPLETE

- **Change:** `2026-09-13-text-to-speech`
- **Target Specification:** `openspec/specs/text-to-speech/spec.md`
- **Execution Date:** 2026-09-13
- **TDD Mode:** Strict TDD (RED -> GREEN -> REFACTOR)

---

## Completed Tasks

### Work Unit 1: Domain Port & Core Definitions (`internal/core`)
- [x] Defined `Synthesizer` interface in `internal/core/port_synthesizer.go`.
- [x] Added unit tests in `internal/core/synthesizer_test.go`.
- [x] Verified: `go test -race ./internal/core/...` PASS.

### Work Unit 2: Audio Synthesis Adapters (`internal/adapters/audio`)
- [x] Implemented `OpenAITTS` in `internal/adapters/audio/openai.go`.
- [x] Implemented `ElevenLabsTTS` in `internal/adapters/audio/elevenlabs.go`.
- [x] Implemented factory `NewSynthesizer` in `internal/adapters/audio/factory.go`.
- [x] Wrote unit tests in `internal/adapters/audio/audio_test.go`.
- [x] Verified: `go test -race ./internal/adapters/audio/...` PASS.

### Work Unit 3: Configuration Schema & Defaults (`internal/config`)
- [x] Added `TTSConfig` struct to `internal/config/config.go`.
- [x] Added `TTS` field to `MultimodalConfig` and root `Config`.
- [x] Implemented default population and synchronization in `applyDefaults`.
- [x] Added unit tests in `internal/config/config_test.go`.
- [x] Verified: `go test -race ./internal/config/...` PASS.

### Work Unit 4: Gateway Voice Sending & Multiplexer Integration (`internal/gateway`, `cmd/agis`)
- [x] Defined `VoiceSender` interface and `HasAudio()` helper in `internal/gateway/adapter.go`.
- [x] Implemented `SendVoice` on `TelegramAdapter` in `internal/gateway/telegram.go`.
- [x] Implemented `SendVoice` on `WhatsAppAdapter` in `internal/gateway/whatsapp.go`.
- [x] Integrated `WithMultiplexerSynthesizer` and automatic voice replies in `Multiplexer.HandleEvent`.
- [x] Wired `Synthesizer` in `cmd/agis/gateway.go`.
- [x] Wrote integration tests in `internal/gateway/voice_test.go`.
- [x] Verified: `go test -race ./internal/gateway/...` PASS.

### Work Unit 5: Verification, Documentation & Roadmap
- [x] Updated `docs/development/hermes-parity-roadmap.md` marking Fase 11 as Shipped.
- [x] Updated `docs/gateway.md` and `docs/configuration.md`.
- [x] Verified full test suite: `go test -race ./...` (28 packages PASS) and `go vet ./...` (0 issues).
