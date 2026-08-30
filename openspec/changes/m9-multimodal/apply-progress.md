# Apply Progress: M9 — Multimodal Ingestion (Vision & Audio)

## Execution Summary

- **Change**: `m9-multimodal`
- **Delivery Strategy**: `auto-chain` (Chained PRs: stacked-to-main)
- **Current Slice Completed**: `PR 2 — Vision Multipart Formatter & Whisper Transcriber Adapter`
- **TDD Mode**: Strict TDD Active (RED → GREEN → TRIANGULATE → REFACTOR)
- **Cumulative Line Count**: ~520 lines across PR 1 and PR 2 (PR 2 is ~255 lines, well within individual 400-line review budget)

---

## Completed Tasks

### PR Slice 1: Core Attachment Model, Config Extensions & Migration 0007
- [x] **1.1 Core Domain**: Added `Attachment` struct (`Type string`, `MimeType string`, `Data []byte`, `URL string`, `Name string`) to `internal/core/types.go` and embedded `Attachments []Attachment` on `core.Message` with `json:"attachments,omitempty"`. Backward compatible with text-only turns.
- [x] **1.2 Config Extensions**: Added `MultimodalConfig`, `VisionConfig`, and `AudioConfig` to `internal/config/config.go` with safe defaults (`enabled: false`, `vision.model: "llama3.2-vision"`, `vision.max_image_size_mb: 10`, `audio.provider: "openai"`, `audio.model: "whisper-1"`, `audio.max_audio_size_mb: 25`).
- [x] **1.3 Config Tests**: Added unit tests in `internal/config/config_test.go` verifying multimodal default resolution and partial/explicit YAML overrides.
- [x] **1.4 Migration 0007**: Created `internal/memory/migrations/0007_attachments.sql` with table `attachments` referencing `messages(id)` with `ON DELETE CASCADE` and index `idx_attachments_msg`.
- [x] **1.5 SQLite Attachment Persistence**: Extended `internal/memory/sqlite.go` in `AppendMessage` to persist `msg.Attachments` transactionally with the message insert, and in `Messages` to populate attachments efficiently for retrieved messages.
- [x] **1.6 Memory Unit Tests**: Implemented comprehensive unit tests in `internal/memory/attachments_test.go` and updated `internal/memory/migrations_test.go` verifying binary BLOB roundtrip, cascade deletion, migration idempotency, mixed messages tail limit handling, and leak checks with `goleak`.

### PR Slice 2: Vision Multipart Formatter & Whisper Transcriber Adapter
- [x] **2.1 Transcriber Port**: Created `internal/core/port_transcriber.go` defining the `Transcriber` interface (`Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)`).
- [x] **2.2 Vision Payload Transformation**: Extended `internal/adapters/llm/client.go` to format messages with image attachments as OpenAI/Ollama-compatible vision multipart content arrays (`text` + `image_url` with Data URL or remote URL) and strict MIME validation (`image/png`, `image/jpeg`, `image/webp`, `image/gif`), preserving standard string serialization for text-only messages.
- [x] **2.3 Whisper Transcription Adapter**: Implemented `internal/adapters/llm/whisper.go` with `multipart/form-data` requests to `/v1/audio/transcriptions`, MIME filename extension deduction, authorization header, and descriptive error handling.
- [x] **2.4 Vision & Whisper Unit Tests**: Implemented comprehensive tests in `internal/adapters/llm/vision_test.go` and `internal/adapters/llm/whisper_test.go` using `httptest.Server`, covering Data URLs, remote URLs, MIME filtering, backward compatibility, streaming, empty audio validation, context cancellation, HTTP 400/401/500 errors, and `goleak.VerifyNone`.

---

## TDD Cycle Evidence

| Phase | Test File / Target | Test Description | Result |
|---|---|---|---|
| RED | `internal/core/transcriber_test.go` | `TestTranscriber_Interface` | Failed (undefined `core.Transcriber`) |
| GREEN | `internal/core/port_transcriber.go` | Created `Transcriber` interface | Passed |
| RED | `internal/adapters/llm/vision_test.go` | `TestVision_MultipartPayload_BinaryDataURL`, `TestVision_MultipartPayload_RemoteURL`, `TestVision_EmptyTextWithImage` | Failed (cannot unmarshal string into multipart parts) |
| GREEN | `internal/adapters/llm/client.go` | Implemented `formatContent`, `isAllowedVisionMIME`, `contentPart`, `imageURLPart` | Passed |
| TRIANGULATE | `internal/adapters/llm/vision_test.go` | `TestVision_MIMEValidation`, `TestVision_TextOnly_BackwardCompatibility`, `TestVision_MultipleImages`, `TestVision_StreamWithImages` | Passed |
| RED | `internal/adapters/llm/whisper_test.go` | `TestWhisper_ImplementsTranscriber`, `TestWhisper_Transcribe_Success`, `TestWhisper_Transcribe_HTTPErrors` | Failed (undefined `Whisper`, `NewWhisper`) |
| GREEN | `internal/adapters/llm/whisper.go` | Implemented `Whisper`, `NewWhisper`, `Transcribe`, `deduceFilename` | Passed |
| TRIANGULATE | `internal/adapters/llm/whisper_test.go` | `TestWhisper_Transcribe_MIMEFilenameDeduction` (table-driven), `TestWhisper_Transcribe_EmptyAudio`, `TestWhisper_Transcribe_ContextCanceled`, `goleak.VerifyNone` | Passed |
| REFACTOR | `internal/adapters/llm/client.go`, `whisper.go` | Clean MIME switch helper and shared HTTP status error handler | Passed |

---

## Files Changed

- `internal/core/port_transcriber.go` (New `core.Transcriber` interface)
- `internal/core/transcriber_test.go` (Unit tests for `Transcriber` port interface)
- `internal/adapters/llm/client.go` (Added vision multipart formatter, `isAllowedVisionMIME`, Data URL formatting)
- `internal/adapters/llm/vision_test.go` (Unit tests for vision multipart transformation and backward compatibility)
- `internal/adapters/llm/whisper.go` (Whisper audio transcription adapter implementing `core.Transcriber`)
- `internal/adapters/llm/whisper_test.go` (Unit and mock tests for Whisper transcription, error handling, and goroutine leaks)
- `openspec/changes/m9-multimodal/tasks.md` (Updated checkboxes for PR 2)
- `openspec/changes/m9-multimodal/apply-progress.md` (Cumulative progress tracking for PR 1 & PR 2)

---

## Test Commands Run

- `go test -count=1 ./...` — PASS (All packages passing)
- `go test -race ./...` — PASS (Zero data races, zero goroutine leaks)

---

## Remaining Tasks (PR Slice 3)

### PR Slice 3: Gateway Media Ingestion (Telegram & Discord), CLI Wiring & Docs
- [ ] Implement `internal/gateway/media.go` with robust download helpers, timeout wrappers, MIME sniffing (`http.DetectContentType`), and size guards (10MB image, 25MB audio). <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/telegram.go` to download photos via `getFile` and voice/audio notes, transcribe audio via `core.Transcriber`, and construct `MessageEvent.Attachments`. <!-- sdd-owner: implementation -->
- [ ] Extend `internal/gateway/discord.go` to download CDN attachments for images and audio, transcribe voice messages, and attach to `MessageEvent`. <!-- sdd-owner: implementation -->
- [ ] Wire `Transcriber` and multimodal options into `cmd/agis/main.go`, `cmd/agis/gateway.go`, and runtime startup flows. <!-- sdd-owner: implementation -->
- [ ] Implement integration tests in `cmd/agis/multimodal_integration_test.go` and `internal/gateway/media_test.go` with race detector (`go test -race ./...`) and `goleak`. <!-- sdd-owner: implementation -->
- [ ] Create `docs/multimodal.md` and update `docs/architecture.md`, `docs/configuration.md`, `docs/gateway.md`, `docs/roadmap.md` (M9 marked as DONE), and `README.md`. <!-- sdd-owner: implementation -->
