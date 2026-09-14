# Design: Text-to-Speech (TTS) Outbound Voice Synthesis

## 1. Architectural Architecture & Component Diagram

```
+-------------------------------------------------------------------+
|                        internal/core                              |
|                                                                   |
|   Transcriber (STT)                      Synthesizer (TTS)        |
|   +Transcribe(audio) string              +Synthesize(text) []byte |
+-------------------------------------------------------------------+
             ^                                      ^
             |                                      |
+------------+--------------------+   +-------------+---------------+
|     internal/adapters/llm       |   |   internal/adapters/audio   |
|   Whisper STT                   |   |   OpenAITTS, ElevenLabsTTS  |
|                                 |   |   NewSynthesizer(...)       |
+---------------------------------+   +-----------------------------+
                                                    |
                                                    v
                                      +-----------------------------+
                                      |      internal/gateway       |
                                      |   Multiplexer               |
                                      |   VoiceSender interface     |
                                      |   TelegramAdapter.SendVoice |
                                      |   WhatsAppAdapter.SendVoice |
                                      +-----------------------------+
```

## 2. Audio Adapter Package (`internal/adapters/audio`)
We establish a dedicated `internal/adapters/audio` package for all speech synthesis providers.

### OpenAITTS Struct & Options
```go
type OpenAITTS struct {
    baseURL    string
    apiKey     string
    model      string
    voice      string
    format     string
    speed      float64
    httpClient *http.Client
}
```
Supported formats: `mp3` (`audio/mpeg`), `opus` (`audio/opus`), `aac` (`audio/aac`), `flac` (`audio/flac`), `wav` (`audio/wav`). Default is `mp3`.
For local engines like Kokoro or LocalAI, setting `base_url: http://localhost:8880/v1` routes to their OpenAI-compatible `/audio/speech` endpoint seamlessly.

### ElevenLabsTTS Struct & Options
```go
type ElevenLabsTTS struct {
    baseURL    string
    apiKey     string
    modelID    string
    voiceID    string
    httpClient *http.Client
}
```
Calls `POST https://api.elevenlabs.io/v1/text-to-speech/{voice_id}`.

## 3. Gateway VoiceSender Interface
In `internal/gateway/adapter.go`:
```go
type VoiceSender interface {
    SendVoice(ctx context.Context, target string, audio []byte, mimeType string, caption string) error
}
```
Adapters that do not support voice sending simply do not implement this interface.
In `Multiplexer`:
Add `WithSynthesizer(s core.Synthesizer) MultiplexerOption`.
In `HandleEvent`:
- If incoming message has an audio attachment:
  - If `m.synthesizer != nil` and adapter implements `VoiceSender`:
    - Call `m.synthesizer.Synthesize(ctx, replyText)`
    - Call `vs.SendVoice(ctx, ev.ChatID, audio, mime, caption)`
    - If either fails, log warning and call `m.Send(ctx, ev.Adapter, ev.ChatID, replyText)`
  - Otherwise, send text via `m.Send`.

## 4. Configuration Schema
In `internal/config/config.go`:
```go
type TTSConfig struct {
    Enabled  bool    `yaml:"enabled"`
    Provider string  `yaml:"provider"`
    Model    string  `yaml:"model"`
    Voice    string  `yaml:"voice"`
    Format   string  `yaml:"format"`
    Speed    float64 `yaml:"speed"`
    BaseURL  string  `yaml:"base_url,omitempty"`
    APIKey   string  `yaml:"api_key,omitempty"`
}
```
Default values:
- `Provider`: `"openai"`
- `Model`: `"tts-1"`
- `Voice`: `"alloy"`
- `Format`: `"mp3"`
- `Speed`: `1.0`
