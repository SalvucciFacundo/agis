# Exploration: Multimodal Ingestion (Vision & Audio)

## Overview
This exploration investigates implementing multimodal ingestion (Vision & Audio) in AGIS. This involves extending the core domain models, LLM provider payloads, audio transcription protocols, and database storage to support attaching non-textual data to messages.

## 1. Core Domain & Types
To support multimodal turns, we need to extend `core.Message` and `core.ChatRequest`.
- `Attachment` struct:
  ```go
  type Attachment struct {
      Type     string // "image" or "audio"
      MimeType string
      Data     []byte
      URL      string // For external CDN/storage
      Name     string
  }
  ```
- `core.Message` should embed `Attachments []Attachment`.
- The `Brain.Step` logic needs to ensure these attachments are propagated to the `Provider`.

## 2. Vision in LLM Provider Adapters
- OpenAI supports vision via multipart content schema:
  `[{"type":"text","text":"..."},{"type":"image_url","image_url":{"url":"data:<mime>;base64,<data>"}}]`
- Adapters must transform `core.Message` with attachments into this format.
- MIME type detection for `image/png`, `image/jpeg`, `image/webp`, `image/gif`.

## 3. Audio Transcription Port
- Need a new port `Transcriber` in `internal/core/` (or similar).
- `Transcribe(ctx, audio []byte, mimeType string) (string, error)`.
- OpenAI adapter: use `/v1/audio/transcriptions`.

## 4. Gateway Media Ingestion
- Need to extend `internal/gateway/` adapters (Telegram/Discord) to:
  - Handle media downloads.
  - Apply file size limits (e.g., 10MB images, 25MB audio).
  - Pass the downloaded data/URL to the `Brain`.

## 5. Database Storage (0007_attachments.sql)
- Need an `attachments` table linked to `messages`.
- `id TEXT PRIMARY KEY`, `message_id TEXT NOT NULL`, `type TEXT NOT NULL`, `mime_type TEXT NOT NULL`, `data BLOB`, `created_at TEXT`.

## Architectural Decisions
1. **Attachment Storage**: Storing BLOBs directly in SQLite for simplicity vs. S3/filesystem. (Decision: Store in SQLite for local-first simplicity, keep BLOBs small).
2. **Transcription**: Do we transcribe on-the-fly? (Decision: Yes, transcribe during ingestion, pass transcript as text).
3. **MIME Detection**: Use `http.DetectContentType`.
4. **Gateway Protocol**: Unified `Attachment` interface in `gateway.MessageEvent`.
5. **Brain Integration**: `Brain.Step` loop needs to handle multi-part messages.
