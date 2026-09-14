# Specification: Text-to-Speech (TTS) Outbound Voice Synthesis

## Scope & Purpose
Defines domain ports, adapters, configuration schemas, and gateway delivery mechanisms for outbound speech synthesis in AGIS.

---

## 1. Domain Port: `internal/core`

### `AGIS-TTS-001`: Synthesizer Contract
- **Requirement**: Package `core` MUST define a port `Synthesizer` for transforming text strings into synthesized audio streams.
- **Given** valid textual input `text` and active `context.Context`,
  **When** `Synthesize(ctx, text)` is invoked,
  **Then** it MUST return raw audio bytes `[]byte`, the MIME type string (e.g., `audio/mpeg` or `audio/ogg`), or an error.
- **Given** empty text or cancelled context,
  **When** `Synthesize(ctx, "")` is called,
  **Then** it MUST return an error without making external network calls.

---

## 2. Adapters: `internal/adapters/audio`

### `AGIS-TTS-002`: OpenAI TTS Adapter (`OpenAITTS`)
- **Requirement**: `OpenAITTS` MUST implement `core.Synthesizer` targeting the OpenAI `/v1/audio/speech` endpoint or compatible server.
- **Given** configured `baseURL`, `apiKey`, `model` (default: `tts-1`), `voice` (default: `alloy`), `format` (default: `mp3`), and `speed` (default: `1.0`),
  **When** `Synthesize` is called with text,
  **Then** it MUST issue a JSON POST request to `{baseURL}/audio/speech` with `Authorization: Bearer <apiKey>` and return the response body bytes with matching MIME type (`audio/mpeg` for mp3, `audio/opus` for opus, `audio/aac` for aac).
- **Given** an HTTP status >= 400,
  **When** the upstream API returns an error response,
  **Then** `Synthesize` MUST return an error containing the status code and error details.

### `AGIS-TTS-003`: ElevenLabs Adapter (`ElevenLabsTTS`)
- **Requirement**: `ElevenLabsTTS` MUST implement `core.Synthesizer` targeting the ElevenLabs REST API.
- **Given** configured `baseURL` (default: `https://api.elevenlabs.io`), `apiKey`, `voiceID`, and `modelID` (default: `eleven_multilingual_v2`),
  **When** `Synthesize` is called with text,
  **Then** it MUST issue a POST request to `{baseURL}/v1/text-to-speech/{voiceID}` with header `xi-api-key: <apiKey>` and return the audio bytes with MIME type `audio/mpeg`.

### `AGIS-TTS-004`: Synthesizer Factory
- **Requirement**: `NewSynthesizer(cfg config.TTSConfig, fallbackAPIKey string)` MUST instantiate the appropriate `core.Synthesizer` implementation based on `cfg.Provider` (`openai`, `elevenlabs`, `kokoro`).
- If `cfg.Provider` is empty or `"openai"` or `"kokoro"`, instantiate `OpenAITTS`.
- If `cfg.Provider` is `"elevenlabs"`, instantiate `ElevenLabsTTS`.
- If `cfg.APIKey` is empty, fall back to `fallbackAPIKey`.

---

## 3. Configuration: `internal/config`

### `AGIS-TTS-005`: TTS Configuration Schema
- **Requirement**: Struct `TTSConfig` MUST support:
  - `enabled: bool` (default: false)
  - `provider: string` (default: `"openai"`)
  - `model: string` (default: `"tts-1"`)
  - `voice: string` (default: `"alloy"`)
  - `format: string` (default: `"mp3"`)
  - `speed: float64` (default: `1.0`)
  - `base_url: string` (optional override)
  - `api_key: string` (optional override)
- **Requirement**: Struct `Config` MUST expose `TTS TTSConfig` both under `Multimodal.TTS` and top-level `TTS`.
- Default values MUST be populated during `config.Load`.

---

## 4. Gateway Integration: `internal/gateway`

### `AGIS-TTS-006`: VoiceSender Port
- **Requirement**: Package `gateway` MUST define optional interface `VoiceSender`:
  ```go
  type VoiceSender interface {
      SendVoice(ctx context.Context, target string, audio []byte, mimeType string, caption string) error
  }
  ```
- Any adapter supporting voice notes MUST implement `VoiceSender`.

### `AGIS-TTS-007`: Telegram Voice Delivery
- **Requirement**: `TelegramAdapter` MUST implement `VoiceSender`.
- **Given** target chat ID, audio bytes, and optional caption,
  **When** `SendVoice` is called,
  **Then** it MUST issue a `multipart/form-data` POST request to `{baseURL}/bot{token}/sendVoice` containing `chat_id`, `voice` form file, and `caption`.

### `AGIS-TTS-008`: WhatsApp Voice Delivery
- **Requirement**: `WhatsAppAdapter` MUST implement `VoiceSender`.
- **Given** target recipient phone number, audio bytes, and MIME type,
  **When** `SendVoice` is called,
  **Then** it MUST upload audio to `{baseURL}/{phone_id}/media` and subsequently send an audio message payload to `{baseURL}/{phone_id}/messages`.

### `AGIS-TTS-009`: Multiplexer Auto-Voice Reply
- **Requirement**: In `Multiplexer.HandleEvent`, if the incoming message contains an audio attachment (`ev.HasAudio()` or voice event) AND a `core.Synthesizer` is configured in the multiplexer:
  1. Synthesize assistant `replyText` using `Synthesizer.Synthesize`.
  2. If the originating adapter implements `VoiceSender`, call `SendVoice`.
  3. If synthesis or `SendVoice` fails, fall back gracefully to text `Send`.
