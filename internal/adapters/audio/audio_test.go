package audio_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/adapters/audio"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestOpenAITTS_Synthesize(t *testing.T) {
	ctx := context.Background()

	t.Run("successful mp3 synthesis", func(t *testing.T) {
		expectedAudio := []byte("mock-mp3-binary-data")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/audio/speech" {
				t.Errorf("expected /audio/speech, got %s", r.URL.Path)
			}
			auth := r.Header.Get("Authorization")
			if auth != "Bearer sk-test-key" {
				t.Errorf("expected Bearer sk-test-key, got %s", auth)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
			}

			var reqBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				t.Fatalf("decoding request body: %v", err)
			}
			if reqBody["input"] != "Hello from AGIS" {
				t.Errorf("expected input 'Hello from AGIS', got %v", reqBody["input"])
			}
			if reqBody["model"] != "tts-1" {
				t.Errorf("expected model tts-1, got %v", reqBody["model"])
			}
			if reqBody["voice"] != "alloy" {
				t.Errorf("expected voice alloy, got %v", reqBody["voice"])
			}
			if reqBody["response_format"] != "mp3" {
				t.Errorf("expected response_format mp3, got %v", reqBody["response_format"])
			}

			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(expectedAudio)
		}))
		defer server.Close()

		client := audio.NewOpenAITTS(audio.OpenAITTSOptions{
			BaseURL: server.URL,
			APIKey:  "sk-test-key",
			Model:   "tts-1",
			Voice:   "alloy",
			Format:  "mp3",
		})

		data, mime, err := client.Synthesize(ctx, "Hello from AGIS")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != core.MimeTypeMP3 {
			t.Errorf("expected MIME %q, got %q", core.MimeTypeMP3, mime)
		}
		if string(data) != string(expectedAudio) {
			t.Errorf("expected audio %q, got %q", string(expectedAudio), string(data))
		}
	})

	t.Run("custom opus format and speed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var reqBody map[string]any
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			if reqBody["response_format"] != "opus" {
				t.Errorf("expected response_format opus, got %v", reqBody["response_format"])
			}
			if reqBody["speed"] != 1.25 {
				t.Errorf("expected speed 1.25, got %v", reqBody["speed"])
			}

			w.Header().Set("Content-Type", "audio/ogg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("opus-audio"))
		}))
		defer server.Close()

		client := audio.NewOpenAITTS(audio.OpenAITTSOptions{
			BaseURL: server.URL,
			APIKey:  "sk-test",
			Format:  "opus",
			Speed:   1.25,
		})

		data, mime, err := client.Synthesize(ctx, "Testing opus")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != core.MimeTypeOpus {
			t.Errorf("expected MIME %q, got %q", core.MimeTypeOpus, mime)
		}
		if string(data) != "opus-audio" {
			t.Errorf("expected audio 'opus-audio', got %q", string(data))
		}
	})

	t.Run("empty text error", func(t *testing.T) {
		client := audio.NewOpenAITTS(audio.OpenAITTSOptions{
			APIKey: "sk-test",
		})
		_, _, err := client.Synthesize(ctx, "   ")
		if err == nil {
			t.Fatal("expected error for empty text, got nil")
		}
	})

	t.Run("http error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key"}}`))
		}))
		defer server.Close()

		client := audio.NewOpenAITTS(audio.OpenAITTSOptions{
			BaseURL: server.URL,
			APIKey:  "invalid-key",
		})

		_, _, err := client.Synthesize(ctx, "Hello")
		if err == nil {
			t.Fatal("expected error for HTTP 401, got nil")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected error to mention 401, got: %v", err)
		}
	})
}

func TestElevenLabsTTS_Synthesize(t *testing.T) {
	ctx := context.Background()

	t.Run("successful synthesis", func(t *testing.T) {
		expectedAudio := []byte("elevenlabs-audio-bytes")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/voice-123") {
				t.Errorf("expected path /v1/text-to-speech/voice-123, got %s", r.URL.Path)
			}
			if r.Header.Get("xi-api-key") != "xi-test-secret" {
				t.Errorf("expected xi-api-key 'xi-test-secret', got %s", r.Header.Get("xi-api-key"))
			}

			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if payload["text"] != "Hello from ElevenLabs" {
				t.Errorf("expected text 'Hello from ElevenLabs', got %v", payload["text"])
			}

			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(expectedAudio)
		}))
		defer server.Close()

		client := audio.NewElevenLabsTTS(audio.ElevenLabsOptions{
			BaseURL: server.URL,
			APIKey:  "xi-test-secret",
			VoiceID: "voice-123",
			ModelID: "eleven_multilingual_v2",
		})

		data, mime, err := client.Synthesize(ctx, "Hello from ElevenLabs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != core.MimeTypeMP3 {
			t.Errorf("expected MIME %q, got %q", core.MimeTypeMP3, mime)
		}
		if string(data) != string(expectedAudio) {
			t.Errorf("expected audio %q, got %q", string(expectedAudio), string(data))
		}
	})

	t.Run("error response handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte("quota exceeded"))
		}))
		defer server.Close()

		client := audio.NewElevenLabsTTS(audio.ElevenLabsOptions{
			BaseURL: server.URL,
			APIKey:  "xi-test",
			VoiceID: "voice-123",
		})

		_, _, err := client.Synthesize(ctx, "Hello")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "402") {
			t.Errorf("expected error to mention 402, got: %v", err)
		}
	})
}

func TestNewSynthesizer_Factory(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		synth := audio.NewSynthesizer(config.TTSConfig{Enabled: false}, "fallback-key")
		if synth != nil {
			t.Errorf("expected nil for disabled config, got %v", synth)
		}
	})

	t.Run("openai provider default", func(t *testing.T) {
		synth := audio.NewSynthesizer(config.TTSConfig{
			Enabled:  true,
			Provider: "openai",
		}, "fallback-key")
		if synth == nil {
			t.Fatal("expected non-nil synthesizer")
		}
		if _, ok := synth.(*audio.OpenAITTS); !ok {
			t.Errorf("expected *audio.OpenAITTS, got %T", synth)
		}
	})

	t.Run("elevenlabs provider", func(t *testing.T) {
		synth := audio.NewSynthesizer(config.TTSConfig{
			Enabled:  true,
			Provider: "elevenlabs",
			Voice:    "rachel",
			APIKey:   "xi-key",
		}, "")
		if synth == nil {
			t.Fatal("expected non-nil synthesizer")
		}
		if _, ok := synth.(*audio.ElevenLabsTTS); !ok {
			t.Errorf("expected *audio.ElevenLabsTTS, got %T", synth)
		}
	})

	t.Run("kokoro provider routes to OpenAITTS", func(t *testing.T) {
		synth := audio.NewSynthesizer(config.TTSConfig{
			Enabled:  true,
			Provider: "kokoro",
			BaseURL:  "http://localhost:8880/v1",
		}, "")
		if synth == nil {
			t.Fatal("expected non-nil synthesizer")
		}
		if _, ok := synth.(*audio.OpenAITTS); !ok {
			t.Errorf("expected *audio.OpenAITTS, got %T", synth)
		}
	})
}
