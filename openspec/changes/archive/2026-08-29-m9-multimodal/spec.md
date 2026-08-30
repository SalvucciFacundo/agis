# M9 — Multimodal Ingestion (Vision & Audio) (Delta Spec)

Delta specification for `m9-multimodal`: Core Attachment Model (`core.Attachment`), Vision Multipart Transformations, Audio Transcriber Port (`core.Transcriber`), OpenAI Whisper Adapter, Telegram & Discord Media Ingestion with Guardrails, SQLite Attachments Storage (`0007_attachments.sql`), and Multimodal Configuration Schema.

---

## multimodal (NEW)

### Requirement AGIS-M9-MM-001: Attachment Domain Model
The system MUST provide a first-class `Attachment` domain model in `internal/core/types.go` to represent non-textual message payloads:
1. `Attachment` struct MUST include the following fields:
   - `Type string`: Categorical media type, strictly `"image"` or `"audio"`.
   - `MimeType string`: Standardized MIME type string (e.g., `"image/png"`, `"image/jpeg"`, `"image/webp"`, `"image/gif"`, `"audio/ogg"`, `"audio/wav"`, `"audio/mpeg"`).
   - `Data []byte`: Raw binary media payload.
   - `URL string`: Optional external CDN, storage, or file URL.
   - `Name string`: Optional original filename or resource label.
2. `core.Message` MUST embed `Attachments []Attachment` with `json:"attachments,omitempty"`.
3. `core.ChatRequest` and internal execution pipelines MUST propagate message attachments through the conversation lifecycle and into LLM provider requests.
4. Messages without attachments MUST retain full backward compatibility with `Attachments` set to `nil` or an empty slice.

#### Scenario: Attachment created with binary payload and metadata
- GIVEN a binary image buffer of PNG data with filename `"screenshot.png"`
- WHEN an `Attachment` struct is instantiated with `Type: "image"`, `MimeType: "image/png"`, `Data: buffer`, `Name: "screenshot.png"`
- THEN the attachment fields are accessible and retain the byte contents and MIME metadata

#### Scenario: Message embeds multiple media attachments
- GIVEN a user message with text `"Analyze these charts"` and two image attachments
- WHEN attached to `core.Message.Attachments`
- THEN `len(message.Attachments)` equals 2 and both attachments preserve their individual binary data and MIME types

#### Scenario: Text-only message backward compatibility
- GIVEN a standard text-only message without attachments
- WHEN processed by core message handlers
- THEN `Attachments` is nil or empty and message processing executes identically to text-only turns

---

### Requirement AGIS-M9-MM-002: Vision Multipart Content Transformation
The system MUST transform messages containing image attachments into provider-compatible vision multipart content schemas in `internal/adapters/llm/`:
1. When image attachments are present on a `core.Message`, the adapter MUST format the prompt content as a multipart array of parts:
   - Text parts MUST be formatted as `{"type": "text", "text": "<content>"}`.
   - Image parts with binary `Data` MUST be formatted as base64 Data URLs: `{"type": "image_url", "image_url": {"url": "data:<mime_type>;base64,<base64_encoded_data>"}}`.
   - Image parts with empty `Data` and a non-empty `URL` MUST use the remote URL: `{"type": "image_url", "image_url": {"url": "<url>"}}`.
2. The transformation MUST validate that image MIME types belong to supported vision formats (`image/png`, `image/jpeg`, `image/webp`, `image/gif`). Unsupported image MIME types MUST be omitted or return an error before making upstream API calls.
3. Non-image attachments (e.g., audio) MUST NOT be serialized into the vision `image_url` array.
4. Messages containing only text MUST continue to serialize as standard string content or single-part text payloads without introducing unnecessary array overhead.

#### Scenario: Message with text and image serialized to vision multipart schema
- GIVEN a `core.Message` containing text `"What is this?"` and a JPEG image attachment with binary data
- WHEN the LLM provider serializes the request payload
- THEN the content field contains a two-element array with a text part and an `image_url` part containing a valid `data:image/jpeg;base64,...` Data URL

#### Scenario: Non-image attachments excluded from vision payload
- GIVEN a `core.Message` containing an audio attachment and text
- WHEN the LLM provider transforms the message for a vision-capable chat completion endpoint
- THEN the audio attachment is not passed as an `image_url` part and text is processed cleanly

#### Scenario: Text-only message retains standard format
- GIVEN a `core.Message` with text `"Hello AGIS"` and no attachments
- WHEN serialized for LLM completion
- THEN the message content is serialized as standard string content

---

### Requirement AGIS-M9-MM-003: Transcriber Port Interface
The system MUST define a dedicated audio transcription port `Transcriber` in `internal/core/port_transcriber.go`:
1. The interface MUST expose:
   ```go
   type Transcriber interface {
       Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
   }
   ```
2. Implementations MUST accept raw audio bytes and MIME type, returning the transcribed text string.
3. If `ctx` is canceled or expires before transcription completes, `Transcribe` MUST return `context.Canceled` or `context.DeadlineExceeded`.
4. If `len(audio) == 0`, `Transcribe` MUST return an error indicating empty audio input without dispatching remote network requests.

#### Scenario: Transcriber processes audio and returns text
- GIVEN a valid audio byte slice of OGG format and an active transcriber
- WHEN `Transcribe(ctx, audioData, "audio/ogg")` is called
- THEN it returns the transcribed transcript string without error

#### Scenario: Context cancellation aborts transcription
- GIVEN a transcriber call with a canceled context
- WHEN `Transcribe(canceledCtx, audioData, "audio/ogg")` is invoked
- THEN it returns `context.Canceled` immediately

#### Scenario: Empty audio slice returns validation error
- GIVEN an empty audio byte slice `[]byte{}`
- WHEN `Transcribe(ctx, emptyData, "audio/wav")` is invoked
- THEN it returns an error and makes no network requests

---

### Requirement AGIS-M9-MM-004: Whisper Audio Transcription Adapter
The system MUST provide an OpenAI Whisper audio transcription adapter in `internal/adapters/llm/` implementing `core.Transcriber`:
1. The adapter MUST construct a `multipart/form-data` HTTP POST request to `/v1/audio/transcriptions` (or custom configured base URL).
2. The multipart request payload MUST contain:
   - Form field `file`: Binary audio content with an appropriate filename and extension deduced from the MIME type (e.g., `audio.ogg` for `audio/ogg`, `audio.wav` for `audio/wav`, `audio.mp3` for `audio/mpeg`).
   - Form field `model`: The configured transcription model name (defaulting to `"whisper-1"`).
3. The request MUST include `Authorization: Bearer <api_key>` (when an API key is configured).
4. On HTTP 200 response, the adapter MUST parse the JSON response body (`{"text": "..."}`) and return the `text` string.
5. On non-200 HTTP responses, the adapter MUST extract the error message from the response body and return a descriptive Go error.

#### Scenario: Audio successfully transcribed via Whisper API
- GIVEN a Whisper transcriber adapter configured with a valid API key and endpoint
- WHEN `Transcribe(ctx, audioBytes, "audio/ogg")` is executed
- THEN it issues a `multipart/form-data` POST request with `model: "whisper-1"` and `file: audio.ogg`, parsing and returning the resulting transcript text

#### Scenario: Whisper API returns non-200 HTTP error
- GIVEN a Whisper transcriber adapter receiving an HTTP 401 Unauthorized or HTTP 400 error
- WHEN `Transcribe(ctx, audioBytes, "audio/ogg")` is executed
- THEN it returns a descriptive error containing the HTTP status code and response body message

---

## gateway (MODIFIED)

### Requirement AGIS-M9-GTW-001: Telegram Photo and Voice Ingestion
The Telegram gateway adapter in `internal/gateway/telegram.go` MUST ingest photo and voice/audio messages:
1. For photo updates (`message.photo`):
   - The adapter MUST select the highest-resolution photo from the `photo` array.
   - It MUST fetch file metadata via the Telegram `getFile` endpoint and download the binary payload via HTTPS within a bounded context timeout.
   - It MUST construct a `core.Attachment` with `Type: "image"`, detected MIME type, and raw data, adding it to the `gateway.MessageEvent`.
   - Any caption accompanying the photo MUST be assigned as the message text.
2. For voice notes (`message.voice`) and audio messages (`message.audio`):
   - The adapter MUST retrieve the file via `getFile` and download the audio payload.
   - When multimodal audio transcription is enabled (`cfg.Multimodal.Audio.Enabled == true`) and a `core.Transcriber` is provided:
     - It MUST invoke `Transcriber.Transcribe` on the downloaded audio.
     - The transcribed text MUST be set as the message text (or prefixed, e.g. `"[Voice transcript]: <text>"`), allowing downstream processing by `core.Brain.Step`.
   - The raw audio payload MUST also be attached as a `core.Attachment` with `Type: "audio"`.
3. If transcription fails or audio is disabled, the adapter MUST log a warning and forward any accompanying caption as text, or report a graceful fallback message.

#### Scenario: Inbound Telegram photo downloaded and passed as attachment
- GIVEN a running Telegram adapter with multimodal enabled
- WHEN an authorized user sends an image with caption `"Check this log graph"`
- THEN the adapter downloads the photo bytes via `getFile`, populates `Attachment.Data`, sets message text to `"Check this log graph"`, and delivers the event to the router

#### Scenario: Inbound Telegram voice note transcribed and enriched
- GIVEN a running Telegram adapter with audio transcription enabled
- WHEN an authorized user sends an OGG voice note
- THEN the adapter downloads the voice note, calls `Transcriber.Transcribe`, populates the message text with the transcription, attaches the audio, and routes the turn to the Brain

---

### Requirement AGIS-M9-GTW-002: Discord Media Ingestion
The Discord gateway adapter in `internal/gateway/discord.go` MUST ingest media attachments from Discord message events:
1. When `message.Attachments` is non-empty on an incoming message:
   - For image attachments (content type or extension matching PNG, JPEG, WebP, GIF):
     - The adapter MUST download the image payload from Discord's CDN URL within a bounded context timeout.
     - It MUST construct a `core.Attachment` with `Type: "image"`, detected MIME type, and binary data.
   - For audio attachments (content type or extension matching OGG, WAV, MP3, M4A):
     - The adapter MUST download the audio payload from Discord's CDN URL.
     - If audio transcription is enabled and a `core.Transcriber` is configured, it MUST transcribe the audio and populate or prefix the message text with the transcript.
     - It MUST attach the audio payload as an `Attachment` with `Type: "audio"`.
2. The aggregated message text and attachments MUST be forwarded to `core.Brain.Step`.

#### Scenario: Inbound Discord image downloaded from CDN and attached
- GIVEN a running Discord adapter with multimodal enabled
- WHEN an authorized user uploads a PNG image with comment `"Architecture sketch"`
- THEN the adapter downloads the image from Discord CDN, attaches it to `MessageEvent.Attachments`, and forwards the event to the Brain

#### Scenario: Inbound Discord audio attachment transcribed
- GIVEN a running Discord adapter with audio transcription enabled
- WHEN an authorized user uploads a `.wav` voice file
- THEN the adapter downloads the audio, executes `Transcriber.Transcribe`, updates the message content with the transcript, and forwards the enriched turn to the Brain

---

### Requirement AGIS-M9-GTW-003: Media Size and MIME Guardrails
Gateway adapters MUST enforce strict size limits and MIME verification guardrails prior to and during media ingestion:
1. **Size Limits**:
   - Images MUST NOT exceed 10MB (`10 * 1024 * 1024` bytes) by default (or `cfg.Multimodal.Vision.MaxImageSizeMB`).
   - Audio payloads MUST NOT exceed 25MB (`25 * 1024 * 1024` bytes) by default (or `cfg.Multimodal.Audio.MaxAudioSizeMB`).
   - Gateways MUST check file metadata / `Content-Length` headers before downloading, and enforce a bounded `io.LimitReader` during download stream processing to fail-closed on oversized content.
2. **MIME Sniffing & Allowed Types**:
   - Downloaded media bytes MUST be verified using `http.DetectContentType(data[:512])` (or format headers).
   - Allowed image types MUST strictly match: `image/png`, `image/jpeg`, `image/webp`, `image/gif`.
   - Allowed audio types MUST strictly match: `audio/ogg`, `audio/wav`, `audio/mpeg`, `audio/mp4`, `audio/x-m4a`.
3. **Fail-Closed Behavior**:
   - Oversized or disallowed media files MUST be rejected immediately, logging a security warning.
   - The gateway SHOULD send a user-friendly notification back to the originating chat platform (e.g. `"Attachment exceeds maximum size limit of 10MB"` or `"Unsupported media type"`).

#### Scenario: Oversized image rejected before processing
- GIVEN a Telegram or Discord photo update indicating a 15MB file size
- WHEN inspected by the gateway media downloader
- THEN the download is aborted, an error is logged, and the payload is not passed to the Brain

#### Scenario: Disallowed MIME type rejected by content sniffing
- GIVEN an attachment claiming to be an image but sniffing as `application/x-executable`
- WHEN content type sniffing runs on the downloaded header bytes
- THEN the attachment is dropped with a security warning and not processed by the vision pipeline

#### Scenario: Valid media within limits passes guardrails
- GIVEN a 2MB PNG image
- WHEN evaluated against size and MIME guardrails
- THEN size check passes, MIME check resolves `image/png`, and the attachment is accepted

---

## repository-memory (MODIFIED)

### Requirement AGIS-M9-REPO-001: Attachments Storage & Migration 0007_attachments.sql
The repository memory system in `internal/memory/` MUST persist message attachments in SQLite:
1. Migration `0007_attachments.sql` MUST create the `attachments` table:
   ```sql
   CREATE TABLE IF NOT EXISTS attachments (
       id         TEXT PRIMARY KEY,
       message_id TEXT NOT NULL,
       type       TEXT NOT NULL,
       mime_type  TEXT NOT NULL,
       data       BLOB NOT NULL,
       name       TEXT,
       url        TEXT,
       created_at TEXT NOT NULL,
       FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
   );
   CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id);
   PRAGMA user_version = 7;
   ```
2. The migration MUST advance `PRAGMA user_version` to `7` idempotently.
3. `Repository.AppendMessage` MUST persist all attachments in `message.Attachments` into the `attachments` table within the same database transaction as the message insert.
4. `Repository.Messages(convID, limit)` and `Repository.GetConversation(ctx, id)` MUST query and reconstruct `Attachments` on returned messages where attachments exist.
5. Deleting a conversation or message MUST cascade and remove all associated rows in the `attachments` table.

#### Scenario: Database migration advances schema to version 7
- GIVEN a database at `user_version = 6`
- WHEN `NewRepository` initializes
- THEN migration `0007_attachments.sql` applies and `PRAGMA user_version` becomes 7

#### Scenario: Message with attachments persisted and retrieved transactionally
- GIVEN a message with two image attachments
- WHEN `AppendMessage` is called
- THEN the message and both attachment records are inserted transactionally, and subsequent `Messages()` call returns the message with matching attachments and binary data

#### Scenario: Cascade deletion removes orphaned attachments
- GIVEN messages with persisted attachments in SQLite
- WHEN the parent message or conversation is deleted
- THEN foreign key cascading deletes all linked rows from the `attachments` table

---

## config-loader (MODIFIED)

### Requirement AGIS-M9-CONF-001: Multimodal Configuration Schema
The configuration loader in `internal/config/config.go` MUST support the following optional root `multimodal` configuration block:

```yaml
multimodal:
  enabled: false
  vision:
    enabled: false
    model: "gpt-4o"
    max_image_size_mb: 10
  audio:
    enabled: false
    provider: "openai" # "openai" (whisper)
    model: "whisper-1"
    max_audio_size_mb: 25
```

1. `multimodal.enabled` MUST default to `false` (opt-in).
2. `multimodal.vision.enabled` MUST default to `false`.
3. `multimodal.audio.enabled` MUST default to `false`.
4. Safe defaults MUST be assigned when fields are omitted:
   - `multimodal.vision.model`: `"gpt-4o"`
   - `multimodal.vision.max_image_size_mb`: `10`
   - `multimodal.audio.provider`: `"openai"`
   - `multimodal.audio.model`: `"whisper-1"`
   - `multimodal.audio.max_audio_size_mb`: `25`
5. An absent `multimodal` block in `config.yaml` MUST preserve complete backward compatibility.

#### Scenario: Default configuration disables multimodal blocks
- GIVEN an empty or minimal `config.yaml`
- WHEN `config.Load()` is executed
- THEN `cfg.Multimodal.Enabled`, `cfg.Multimodal.Vision.Enabled`, and `cfg.Multimodal.Audio.Enabled` are all `false`, with safe defaults populated for model and size limits

#### Scenario: Explicit multimodal configuration loaded
- GIVEN a `config.yaml` with `multimodal.enabled: true`, `multimodal.vision.enabled: true`, and `multimodal.audio.enabled: true`
- WHEN `config.Load()` is executed
- THEN all struct fields, models, and size limits are populated accurately

