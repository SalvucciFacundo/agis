# Multimodal Ingestion Spec

## Purpose

Provide first-class multimodal ingestion for AGIS, defining the `core.Attachment` model, OpenAI/Ollama vision multipart content transformation with Base64 Data URLs, the `core.Transcriber` port, and the OpenAI Whisper audio transcription adapter.

## Requirements

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

---

### Requirement AGIS-M9-MM-002: Vision Multipart Content Transformation
The system MUST transform messages containing image attachments into provider-compatible vision multipart content schemas in `internal/adapters/llm/`:
1. When image attachments are present on a `core.Message`, the adapter MUST format the prompt content as a multipart array of parts:
   - Text parts MUST be formatted as `{"type": "text", "text": "<content>"}`.
   - Image parts with binary `Data` MUST be formatted as base64 Data URLs: `{"type": "image_url", "image_url": {"url": "data:<mime_type>;base64,<base64_encoded_data>"}}`.
   - Image parts with empty `Data` and a non-empty `URL` MUST use the remote URL: `{"type": "image_url", "image_url": {"url": "<url>"}}`.
2. The transformation MUST validate that image MIME types belong to supported vision formats (`image/png`, `image/jpeg`, `image/webp`, `image/gif`). Unsupported image MIME types MUST be omitted or return an error before making upstream API calls.
3. Non-image attachments (e.g., audio) MUST NOT be serialized into the vision `image_url` array.
4. Messages containing only text MUST continue to serialize as standard string content without introducing unnecessary array overhead.

#### Scenario: Message with text and image serialized to vision multipart schema
- GIVEN a `core.Message` containing text `"What is this?"` and a JPEG image attachment with binary data
- WHEN the LLM provider serializes the request payload
- THEN the content field contains a two-element array with a text part and an `image_url` part containing a valid `data:image/jpeg;base64,...` Data URL

#### Scenario: Non-image attachments excluded from vision payload
- GIVEN a `core.Message` containing an audio attachment and text
- WHEN the LLM provider transforms the message for a vision-capable chat completion endpoint
- THEN the audio attachment is not passed as an `image_url` part and text is processed cleanly

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

---

### Requirement AGIS-M9-MM-004: Whisper Audio Transcription Adapter
The system MUST provide an OpenAI Whisper audio transcription adapter in `internal/adapters/llm/` implementing `core.Transcriber`:
1. The adapter MUST construct a `multipart/form-data` HTTP POST request to `/v1/audio/transcriptions` (or custom configured base URL).
2. The multipart request payload MUST contain:
   - Form field `file`: Binary audio content with an appropriate filename and extension deduced from the MIME type (e.g., `audio.ogg` for `audio/ogg`, `audio.wav` for `audio/wav`, `audio.mp3` for `audio/mpeg`).
   - Form field `model`: The configured transcription model name (defaulting to `"whisper-1"`).
3. The request MUST include `Authorization: Bearer <api_key>` (when an API key is configured).
4. The adapter MUST parse JSON responses containing `{"text": "..."}` and return the transcript text string.
5. HTTP status error codes (e.g. 401 Unauthorized, 429 Rate Limit, 500 Server Error) MUST be surfaced as wrapped descriptive Go errors.

#### Scenario: Whisper adapter transcribes voice note
- GIVEN a valid Whisper adapter and OGG audio payload
- WHEN `Transcribe` is executed
- THEN an HTTP POST request is sent to `/v1/audio/transcriptions` and the resulting transcript is returned
