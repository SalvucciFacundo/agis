# Archive Report: Text-to-Speech (TTS) Outbound Voice Synthesis

## Metadata
- **Change Name:** `2026-09-13-text-to-speech`
- **Archived Date:** 2026-09-13
- **Author/Team:** Senior Architect / AGIS Core Team
- **Target Spec:** `openspec/specs/text-to-speech/spec.md`

## Summary of Delivery
Implemented Phase 11: Text-to-Speech (TTS) Outbound Voice Synthesis for AGIS.
- Defined `core.Synthesizer` port in `internal/core`.
- Created package `internal/adapters/audio` implementing `OpenAITTS`, `ElevenLabsTTS`, and `NewSynthesizer` factory.
- Added `TTSConfig` in `internal/config` supporting model, voice, speed, format, base URL, and API key overrides.
- Defined `VoiceSender` interface in `internal/gateway`.
- Implemented `SendVoice` on `TelegramAdapter` (`sendVoice` multipart) and `WhatsAppAdapter` (media upload and audio dispatch).
- Updated `Multiplexer.HandleEvent` to automatically synthesize and deliver voice notes when users interact via audio, with fail-safe fallback to plain text.
- Wired synthesizer into `cmd/agis/gateway.go`.
- Updated documentation in `docs/gateway.md`, `docs/configuration.md`, and roadmap `docs/development/hermes-parity-roadmap.md`.
- 100% test coverage under `go test -race ./...` and 0 issues under `go vet ./...`.
