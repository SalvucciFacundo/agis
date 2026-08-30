# Apply Progress: M9 — Multimodal Ingestion (Vision & Audio)

## Execution Summary

- **Change**: `m9-multimodal`
- **Delivery Strategy**: `auto-chain` (Chained PRs: stacked-to-main)
- **Current Slice**: `PR 1 — Core Attachment Model, Config Extensions & Migration 0007`
- **TDD Mode**: Strict TDD Active (RED → GREEN → TRIANGULATE → REFACTOR)
- **Line Count**: +265 lines (well under 400-line review budget)

---

## Completed Tasks (PR Slice 1)

- [x] **1.1 Core Domain**: Added `Attachment` struct (`Type string`, `MimeType string`, `Data []byte`, `URL string`, `Name string`) to `internal/core/types.go` and embedded `Attachments []Attachment` on `core.Message` with `json:"attachments,omitempty"`. Backward compatible with text-only turns.
- [x] **1.2 Config Extensions**: Added `MultimodalConfig`, `VisionConfig`, and `AudioConfig` to `internal/config/config.go` with safe defaults (`enabled: false`, `vision.model: "llama3.2-vision"`, `vision.max_image_size_mb: 10`, `audio.provider: "openai"`, `audio.model: "whisper-1"`, `audio.max_audio_size_mb: 25`).
- [x] **1.3 Config Tests**: Added unit tests in `internal/config/config_test.go` verifying multimodal default resolution and partial/explicit YAML overrides.
- [x] **1.4 Migration 0007**: Created `internal/memory/migrations/0007_attachments.sql` with table `attachments` referencing `messages(id)` with `ON DELETE CASCADE` and index `idx_attachments_msg`.
- [x] **1.5 SQLite Attachment Persistence**: Extended `internal/memory/sqlite.go` in `AppendMessage` to persist `msg.Attachments` transactionally with the message insert, and in `Messages` to populate attachments efficiently for retrieved messages.
- [x] **1.6 Memory Unit Tests**: Implemented comprehensive unit tests in `internal/memory/attachments_test.go` and updated `internal/memory/migrations_test.go` verifying binary BLOB roundtrip, cascade deletion, migration idempotency, mixed messages tail limit handling, and leak checks with `goleak`.

---

## TDD Cycle Evidence

| Phase | Test File / Target | Test Description | Result |
|---|---|---|---|
| RED | `internal/core/attachment_test.go` | `TestAttachment_DomainModel`, `TestMessage_WithAttachments_JSON` | Failed (undefined types) |
| GREEN | `internal/core/types.go` | Added `Attachment` and `Message.Attachments` | Passed |
| RED | `internal/config/config_test.go` | `TestLoad_MultimodalDefaultsAndExplicit` | Failed (undefined config structs) |
| GREEN | `internal/config/config.go` | Added `MultimodalConfig`, `defaults()`, `applyDefaults()` | Passed |
| RED | `internal/memory/migrations_test.go`, `attachments_test.go` | Migration 0007, table existence, attachment roundtrip, cascade delete | Failed (no table, no persistence) |
| GREEN | `internal/memory/migrations/0007_attachments.sql`, `sqlite.go` | Created 0007 migration, transactional insert in `AppendMessage`, `populateAttachments` in `Messages` | Passed |
| TRIANGULATE | `internal/memory/attachments_test.go` | Multi-message mixed attachments, limit tail pagination, binary BLOB safety, `goleak` verification | Passed |
| REFACTOR | `internal/memory/sqlite.go` | Clean `rowid` ordering for deterministic attachment sequence | Passed |

---

## Files Changed

- `internal/core/types.go` (Added `Attachment` model, embedded `Attachments` in `Message`)
- `internal/core/attachment_test.go` (Domain serialization & backward compatibility tests)
- `internal/config/config.go` (Added `MultimodalConfig`, `VisionConfig`, `AudioConfig`, safe defaults)
- `internal/config/config_test.go` (Unit tests for multimodal configuration loader)
- `internal/memory/migrations/0007_attachments.sql` (DDL for `attachments` table & index)
- `internal/memory/sqlite.go` (Attachment persistence in `AppendMessage`, population in `Messages`)
- `internal/memory/migrations_test.go` (Migration 0007 upgrade & idempotency tests)
- `internal/memory/attachments_test.go` (Attachment storage, retrieval, BLOB roundtrip, cascade tests)
- `openspec/changes/m9-multimodal/tasks.md` (Updated checkboxes for PR 1)

---

## Test Commands Run

- `go test -count=1 ./...` — PASS (All packages passing)
- `go test -race ./...` — PASS (Zero data races, zero goroutine leaks)

---

## Remaining Tasks (PR Slice 2 & 3)

### PR Slice 2: Vision Multipart Formatter & Whisper Transcriber Adapter
- [ ] Create `internal/core/port_transcriber.go` defining the `Transcriber` interface (`Transcribe(ctx, audio, mimeType) (string, error)`). <!-- sdd-owner: implementation -->
- [ ] Extend `internal/adapters/llm/client.go` to format messages with image attachments as OpenAI/Ollama-compatible vision multipart content arrays with base64 Data URLs and strict MIME validation (`image/png`, `image/jpeg`, `image/webp`, `image/gif`). <!-- sdd-owner: implementation -->
- [ ] Implement `internal/adapters/llm/whisper.go` issuing `multipart/form-data` requests to `/v1/audio/transcriptions` with model `"whisper-1"`. <!-- sdd-owner: implementation -->
- [ ] Implement unit and mock tests in `internal/adapters/llm/vision_test.go` and `internal/adapters/llm/whisper_test.go` using `httptest.Server` to verify vision serialization, transcription, error handling, and `goleak.VerifyNone`. <!-- sdd-owner: implementation -->

### PR Slice 3: Gateway Media Ingestion (Telegram & Discord), CLI Wiring & Docs
- [ ] Implement `internal/gateway/media.go` with robust download helpers, timeout wrappers, MIME sniffing (`http.DetectContentType`), and size guards (10MB image, 25MB audio). <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/telegram.go` to download photos via `getFile` and voice/audio notes, transcribe audio via `core.Transcriber`, and construct `MessageEvent.Attachments`. <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/discord.go` to download CDN attachments for images and audio, transcribe voice messages, and attach to `MessageEvent`. <!-- sdd-owner: implementation -->
- [ ] Wire `Transcriber` and multimodal options into `cmd/agis/main.go`, `cmd/agis/gateway.go`, and runtime startup flows. <!-- sdd-owner: implementation -->
- [ ] Implement integration tests in `cmd/agis/multimodal_integration_test.go` and `internal/gateway/media_test.go` with race detector (`go test -race ./...`) and `goleak`. <!-- sdd-owner: implementation -->
- [ ] Create `docs/multimodal.md` and update `docs/architecture.md`, `docs/configuration.md`, `docs/gateway.md`, `docs/roadmap.md` (M9 marked as DONE), and `README.md`. <!-- sdd-owner: implementation -->
