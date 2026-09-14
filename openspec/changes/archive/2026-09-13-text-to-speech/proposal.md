# Change Proposal: Text-to-Speech (TTS) Outbound Voice Synthesis

## 1. Problem Statement & Motivation
AGIS currently supports multimodal perception for inbound audio (speech-to-text via `core.Transcriber` and OpenAI Whisper), enabling users on chat gateways like Telegram and WhatsApp to send voice notes which AGIS transcribes and processes. However, AGIS can only reply with plain text.

To achieve complete parity with Hermes Agent and provide a natural voice interface across messaging platforms, AGIS needs outbound Text-to-Speech (TTS) capabilities. When users interact with AGIS through voice notes or configure a profile for audio responses, AGIS should synthesize its response and send back voice notes on platforms supporting audio playback (Telegram, WhatsApp).

## 2. Proposed Solution
1. **Core Domain Port (`internal/core`)**:
   - Introduce `core.Synthesizer` interface:
     ```go
     type Synthesizer interface {
         Synthesize(ctx context.Context, text string) ([]byte, string, error) // audio payload, mimeType, error
     }
     ```
2. **Audio Adapters (`internal/adapters/audio`)**:
   - Implement `OpenAITTS`: supports OpenAI speech API (`/v1/audio/speech`), compatible with standard models (`tts-1`, `tts-1-hd`), voices (`alloy`, `echo`, `fable`, `onyx`, `nova`, `shimmer`), formats (`mp3`, `opus`, `aac`), and local OpenAI-compatible engines (Kokoro, LocalAI).
   - Implement `ElevenLabsTTS`: supports ElevenLabs API (`/v1/text-to-speech/{voice_id}`) for expressive multilingual voices.
   - Implement factory `NewSynthesizer(cfg config.TTSConfig, fallbackAPIKey string)` to construct the configured synthesizer.
3. **Configuration Subsystem (`internal/config`)**:
   - Add `TTSConfig` supporting `enabled`, `provider`, `model`, `voice`, `format`, `speed`, `base_url`, `api_key`.
   - Embed `TTS` in `MultimodalConfig` (`multimodal.tts`) and expose at top level in `Config.TTS` for ergonomic YAML declaration.
4. **Gateway Outbound Voice Delivery (`internal/gateway`)**:
   - Define `VoiceSender` interface:
     ```go
     type VoiceSender interface {
         SendVoice(ctx context.Context, target string, audio []byte, mimeType string, caption string) error
     }
     ```
   - Implement `SendVoice` on `TelegramAdapter` (`POST /bot<token>/sendVoice`).
   - Implement `SendVoice` on `WhatsAppAdapter` (`POST /<phone-id>/media` + `POST /<phone-id>/messages`).
   - Update `Multiplexer.HandleEvent`: when an inbound message contains audio or voice response mode is triggered, synthesize the assistant reply using the wired `Synthesizer` and deliver via `SendVoice`. Gracefully fall back to text `Send` if synthesis or voice sending fails.

## 3. Risks & Tradeoffs
- **Synthesis Latency**: TTS API calls add latency (~300ms-1s). We mitigate this by executing TTS asynchronously only when needed, and falling back gracefully to text on error or context timeout.
- **Provider Outages**: If the TTS provider errors or rate-limits, the user should never be left without an answer; `Multiplexer` logs a warning and delivers the text response immediately.
- **Cost**: Synthesizing long texts costs API tokens; long responses can be truncated or capped for voice synthesis with full text sent as caption or follow-up.
