# SDD Proposal: m9-multimodal (Multimodal Ingestion)

## 1. Capabilities Contract

- **`multimodal` (NEW)**: Extends the core system to support non-text attachments. Introduces the `core.Attachment` model, multipart payload transformation for OpenAI/Ollama vision, the `core.Transcriber` port, and an OpenAI Whisper transcription adapter.
- **`gateway` (MODIFIED)**: Automated media ingestion for Telegram (photos via `getFile`, voice/audio notes) and Discord (CDN attachments). Implements MIME validation, size limits (10MB image, 25MB audio), and voice-to-text turn enrichment.
- **`repository-memory` (MODIFIED)**: SQLite schema migration `0007_attachments.sql` (`attachments` table linked to `messages`), updates schema to `user_version = 7`, and integrates attachment persistence logic.
- **`config-loader` (MODIFIED)**: Introduces the `MultimodalConfig` schema within the root configuration (`enabled`, `vision`, `audio`) in `internal/config/config.go`.

## 2. Architectural Decisions

- **D1: Universal `Attachment` Model**: Introduce `Attachment` in `internal/core/types.go` (fields: `Type`, `MimeType`, `Data`, `URL`, `Name`). `core.Message` will embed `Attachments []Attachment`.
- **D2: Vision Payload Transformation**: Implement standard OpenAI-compatible multipart content arrays (`[{"type":"text","text":"..."},{"type":"image_url","image_url":{"url":"data:<mime>;base64,<data>"}}]`) for Vision support. This ensures compatibility with GPT-4o, Llama 3.2-Vision, and Claude APIs inside the LLM adapters.
- **D3: Transcription Port**: Define the `core.Transcriber` interface in `internal/core/port_transcriber.go` (`Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)`).
- **D4: Whisper Transcription Adapter**: Implement a Whisper-backed transcriber adapter (leveraging `/v1/audio/transcriptions` multipart protocol).
- **D5: Gateway Downloaders & Limits**: Implement media downloaders in `internal/gateway/` for Telegram and Discord, featuring bounded timeouts, strict MIME sniffing (`http.DetectContentType`), and file size guards.
- **D6: SQLite Schema & Migration**: Introduce `0007_attachments.sql` linking an `attachments` table to `messages` via `message_id`, securely persisting raw BLOBs locally for simplicity.
- **D7: Configuration Layering**: Update `internal/config/config.go` to include `MultimodalConfig`, enabling users to selectively toggle the overarching multimodal feature or specific vision/audio subsystems.
- **D8: Delivery Strategy**: Adopt a chained PR strategy. The workload forecast is ~700-900 lines of change spread across 3 PR slices (Core/DB, LLM/Adapters, Gateway/Config) to respect reviewer burnout limits and guarantee focused code reviews.

## 3. Security & Guardrails

- **Strict Size Limits**: 10MB maximum for image files, 25MB maximum for audio payloads. Gateways enforce this during metadata inspection before engaging in full stream processing.
- **MIME Validation**: Validations against a strictly enforced allowed list (`image/png`, `image/jpeg`, `image/webp`, `image/gif`, `audio/ogg`, `audio/wav`, `audio/mpeg`).
- **Bounded Remote Downloads**: Strict context timeouts will govern all remote CDN and Telegram API downloads, mitigating slow-loris or resource-exhaustion risks.
- **Prompt Injection Scanning**: Transcribed audio logic ensures it is passed as standard, untrusted user text exactly as if it was typed, naturally inheriting existing system prompt guardrails.

## 4. Compatibility & Rollback

- **Additive Backward Compatibility**: Schema additions are strictly additive. Pre-existing text-only turns work identically. Existing LLM provider endpoints seamlessly accept payloads without media attachments.
- **Rollback Mechanism**: Multimodal functionality is fully toggleable via the configuration (`multimodal.enabled: false`). If disabled, gateways cleanly drop or ignore non-text attachments, reverting entirely to standard M1-M8 text-only behavior.