# Tasks: M9 — Multimodal Ingestion (Vision & Audio)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~750 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

```text
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

---

## PR Slice 1: Core Attachment Model, Config Extensions & Migration 0007

- [x] Implement `Attachment` struct in `internal/core/types.go` and embed `Attachments []Attachment` on `core.Message` with backward compatibility for text-only messages. <!-- sdd-owner: implementation -->
- [x] Implement `MultimodalConfig` (`Enabled`, `Vision`, `Audio`) and sub-configs (`VisionConfig`, `AudioConfig`) in `internal/config/config.go` with safe defaults (`enabled: false`). <!-- sdd-owner: implementation -->
- [x] Add unit tests for configuration loading and defaults in `internal/config/config_test.go`. <!-- sdd-owner: implementation -->
- [x] Create SQLite migration `internal/memory/migrations/0007_attachments.sql` establishing the `attachments` table linked to `messages(id)` with cascade delete and `PRAGMA user_version = 7`. <!-- sdd-owner: implementation -->
- [x] Extend `internal/memory/sqlite.go` to persist and load message attachments in transactions (`AppendMessage`, `Messages`, `GetConversation`). <!-- sdd-owner: implementation -->
- [x] Implement unit tests in `internal/memory/attachments_test.go` and `internal/memory/migrations_test.go` verifying attachment persistence, BLOB binary roundtrip, cascade deletion, and migration idempotency. <!-- sdd-owner: implementation -->

---

## PR Slice 2: Vision Multipart Formatter & Whisper Transcriber Adapter

- [ ] Create `internal/core/port_transcriber.go` defining the `Transcriber` interface (`Transcribe(ctx, audio, mimeType) (string, error)`). <!-- sdd-owner: implementation -->
- [ ] Extend `internal/adapters/llm/client.go` to format messages with image attachments as OpenAI/Ollama-compatible vision multipart content arrays with base64 Data URLs and strict MIME validation (`image/png`, `image/jpeg`, `image/webp`, `image/gif`). <!-- sdd-owner: implementation -->
- [ ] Implement `internal/adapters/llm/whisper.go` issuing `multipart/form-data` requests to `/v1/audio/transcriptions` with model `"whisper-1"`. <!-- sdd-owner: implementation -->
- [ ] Implement unit and mock tests in `internal/adapters/llm/vision_test.go` and `internal/adapters/llm/whisper_test.go` using `httptest.Server` to verify vision serialization, transcription, error handling, and `goleak.VerifyNone`. <!-- sdd-owner: implementation -->

---

## PR Slice 3: Gateway Media Ingestion (Telegram & Discord), CLI Wiring & Docs

- [ ] Implement `internal/gateway/media.go` with robust download helpers, timeout wrappers, MIME sniffing (`http.DetectContentType`), and size guards (10MB image, 25MB audio). <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/telegram.go` to download photos via `getFile` and voice/audio notes, transcribe audio via `core.Transcriber`, and construct `MessageEvent.Attachments`. <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/discord.go` to download CDN attachments for images and audio, transcribe voice messages, and attach to `MessageEvent`. <!-- sdd-owner: implementation -->
- [ ] Wire `Transcriber` and multimodal options into `cmd/agis/main.go`, `cmd/agis/gateway.go`, and runtime startup flows. <!-- sdd-owner: implementation -->
- [ ] Implement integration tests in `cmd/agis/multimodal_integration_test.go` and `internal/gateway/media_test.go` with race detector (`go test -race ./...`) and `goleak`. <!-- sdd-owner: implementation -->
- [ ] Create `docs/multimodal.md` and update `docs/architecture.md`, `docs/configuration.md`, `docs/gateway.md`, `docs/roadmap.md` (M9 marked as DONE), and `README.md`. <!-- sdd-owner: implementation -->
