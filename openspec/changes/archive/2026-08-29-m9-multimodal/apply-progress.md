# Apply Progress: M9 — Multimodal Ingestion (Vision & Audio)

## Execution Summary

- **Change**: `m9-multimodal`
- **Delivery Strategy**: `auto-chain` (Chained PRs: stacked-to-main)
- **Current Slice Completed**: `PR 3 — Gateway Media Ingestion (Telegram & Discord), CLI Wiring & Docs` (ALL PR SLICES COMPLETED: PR 1, PR 2, PR 3)
- **TDD Mode**: Strict TDD Active (RED → GREEN → TRIANGULATE → REFACTOR)
- **Cumulative Line Count**: ~850 lines across PR 1, PR 2, and PR 3 (PR 3 is ~330 lines, well within individual 400-line review budget)

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

### PR Slice 3: Gateway Media Ingestion (Telegram & Discord), CLI Wiring & Docs
- [x] **3.1 Gateway Media Helpers**: Implemented `internal/gateway/media.go` with `DownloadMedia`, `SniffContentType`, `IsAllowedImageMime`, and `IsAllowedAudioMime` enforcing 10MB image / 25MB audio size boundaries, `io.LimitReader` stream guards, and MIME content sniffing.
- [x] **3.2 Telegram Ingestion**: Extended `internal/gateway/telegram.go` to download photos via `getFile` (selecting highest resolution), download voice/audio notes, transcribe audio via `core.Transcriber`, and attach media payloads to `MessageEvent.Attachments`.
- [x] **3.3 Discord Ingestion**: Extended `internal/gateway/discord.go` to inspect `attachments`, download image and audio files from Discord CDN URLs with guardrails, transcribe audio files via `core.Transcriber`, and populate `MessageEvent.Attachments`.
- [x] **3.4 Multiplexer & Brain Turn Wiring**: Updated `internal/gateway/multiplexer.go` to support turns with attachments, `internal/core/brain.go` with `StepWithAttachments`, and wired `llm.NewWhisper` into `cmd/agis/gateway.go` when `cfg.Multimodal.Audio.Enabled == true`.
- [x] **3.5 Integration Tests**: Added end-to-end integration tests in `cmd/agis/multimodal_integration_test.go` verifying Telegram photo download -> Vision LLM turn -> SQLite attachment persistence, Telegram voice -> Whisper transcription -> Brain turn, Discord CDN image download -> Brain turn, and leak-free execution under `go test -race ./...` with `goleak.VerifyNone`.
- [x] **3.6 Documentation**: Created `docs/multimodal.md` and updated `docs/architecture.md`, `docs/configuration.md`, `docs/gateway.md`, `docs/roadmap.md` (M9 marked as DONE), and `README.md`.

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
| RED | `internal/gateway/media_test.go` | `TestSniffContentType`, `TestIsAllowedMime`, `TestDownloadMedia_Success`, `TestDownloadMedia_ExceedsContentLength`, `TestDownloadMedia_ExceedsStreamLimit`, `TestDownloadMedia_UnsupportedMime` | Failed (undefined `DownloadMedia`, `SniffContentType`, etc.) |
| GREEN | `internal/gateway/media.go` | Implemented `DownloadMedia`, `SniffContentType`, `IsAllowedImageMime`, `IsAllowedAudioMime`, size & stream guards | Passed |
| RED | `internal/gateway/telegram_multimodal_test.go` | `TestTelegramAdapter_PhotoIngestion`, `TestTelegramAdapter_VoiceIngestion_WithTranscriber` | Failed (undefined `WithTelegramTranscriber`) |
| GREEN | `internal/gateway/telegram.go` | Added `WithTelegramTranscriber`, `getFile`, photo & voice note downloads, audio transcription | Passed |
| RED | `internal/gateway/discord_multimodal_test.go` | `TestDiscordAdapter_ImageAttachmentIngestion`, `TestDiscordAdapter_AudioAttachmentIngestion_WithTranscriber` | Failed (undefined `WithDiscordTranscriber`) |
| GREEN | `internal/gateway/discord.go` | Added `WithDiscordTranscriber`, CDN attachment downloads for images/audio, audio transcription | Passed |
| RED | `cmd/agis/multimodal_integration_test.go` | `TestMultimodalIntegration_TelegramPhotoToVisionBrain`, `TestMultimodalIntegration_TelegramVoiceToWhisperAndBrain`, `TestMultimodalIntegration_DiscordImageAttachment` | Failed (compilation / wiring assertions) |
| GREEN | `cmd/agis/gateway.go`, `internal/core/brain.go`, `internal/gateway/multiplexer.go` | Wired Whisper audio transcriber into gateway options, `StepWithAttachments` on Brain, multiplexer attachment dispatch | Passed |
| REFACTOR | `internal/gateway/media.go`, `telegram.go`, `discord.go` | Cleaned up helper error signatures, size constants, and logger parameters | Passed |

---

## Files Changed

- `internal/gateway/adapter.go` (Added `Attachments []core.Attachment` to `MessageEvent`)
- `internal/gateway/media.go` (New media download helper, MIME sniffing, size limit guards)
- `internal/gateway/media_test.go` (Unit tests for MIME sniffing and download guardrails)
- `internal/gateway/telegram.go` (Added photo & voice/audio ingestion with `getFile` and `core.Transcriber`)
- `internal/gateway/telegram_multimodal_test.go` (Unit tests for Telegram multimodal ingestion)
- `internal/gateway/discord.go` (Added CDN attachment downloads for images/audio and Whisper transcription)
- `internal/gateway/discord_multimodal_test.go` (Unit tests for Discord multimodal ingestion)
- `internal/gateway/multiplexer.go` (Propagated attachments in `HandleEvent` to `Brain.StepWithAttachments`)
- `internal/core/brain.go` (Added `StepWithAttachments` method to `core.Brain`)
- `internal/core/brain_test.go` (Unit test for `Brain.StepWithAttachments`)
- `cmd/agis/gateway.go` (Wired `llm.NewWhisper` into Telegram and Discord gateway startup when audio transcription is enabled)
- `cmd/agis/multimodal_integration_test.go` (End-to-end integration tests for Telegram and Discord multimodal turns)
- `docs/multimodal.md` (New comprehensive guide for vision and voice ingestion)
- `docs/architecture.md` (Updated architecture package layout and core ports)
- `docs/configuration.md` (Documented `multimodal` configuration block)
- `docs/gateway.md` (Added gateway multimodal capabilities section)
- `docs/roadmap.md` (Marked M9 Multimodal Ingestion as DONE)
- `README.md` (Updated milestone badge to M1-M9 Shipped and added multimodal feature highlights)
- `openspec/changes/m9-multimodal/tasks.md` (All checkboxes completed for PR 1, PR 2, PR 3)
- `openspec/changes/m9-multimodal/apply-progress.md` (Comprehensive cumulative apply progress report)

---

## Test Commands Run

- `go test -count=1 ./...` — PASS (All packages passing)
- `go test -race ./...` — PASS (Zero data races, zero goroutine leaks)

---

## Remaining Tasks

None. All PR slices (PR 1, PR 2, PR 3) are 100% complete.
