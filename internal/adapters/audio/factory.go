package audio

import (
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// NewSynthesizer constructs the configured core.Synthesizer based on config.TTSConfig.
// If TTS is disabled, it returns nil.
func NewSynthesizer(cfg config.TTSConfig, fallbackAPIKey string) core.Synthesizer {
	if !cfg.Enabled {
		return nil
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = fallbackAPIKey
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "elevenlabs":
		return NewElevenLabsTTS(ElevenLabsOptions{
			BaseURL: cfg.BaseURL,
			APIKey:  apiKey,
			ModelID: cfg.Model,
			VoiceID: cfg.Voice,
		})
	case "kokoro":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:8880/v1"
		}
		return NewOpenAITTS(OpenAITTSOptions{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   cfg.Model,
			Voice:   cfg.Voice,
			Format:  cfg.Format,
			Speed:   cfg.Speed,
		})
	default: // "openai" or empty
		return NewOpenAITTS(OpenAITTSOptions{
			BaseURL: cfg.BaseURL,
			APIKey:  apiKey,
			Model:   cfg.Model,
			Voice:   cfg.Voice,
			Format:  cfg.Format,
			Speed:   cfg.Speed,
		})
	}
}
