package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

const (
	defaultElevenLabsBaseURL = "https://api.elevenlabs.io"
	defaultElevenLabsModel   = "eleven_multilingual_v2"
	defaultElevenLabsVoice   = "21m00Tcm4TlvDq8ikWAM" // Rachel
)

// ElevenLabsOptions configures the ElevenLabs TTS adapter.
type ElevenLabsOptions struct {
	BaseURL    string
	APIKey     string
	ModelID    string
	VoiceID    string
	HTTPClient *http.Client
}

// ElevenLabsTTS implements core.Synthesizer using ElevenLabs text-to-speech API.
type ElevenLabsTTS struct {
	baseURL    string
	apiKey     string
	modelID    string
	voiceID    string
	httpClient *http.Client
}

var _ core.Synthesizer = (*ElevenLabsTTS)(nil)

// NewElevenLabsTTS constructs a new ElevenLabsTTS synthesizer.
func NewElevenLabsTTS(opts ElevenLabsOptions) *ElevenLabsTTS {
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultElevenLabsBaseURL
	}
	modelID := opts.ModelID
	if modelID == "" {
		modelID = defaultElevenLabsModel
	}
	voiceID := opts.VoiceID
	if voiceID == "" {
		voiceID = defaultElevenLabsVoice
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &ElevenLabsTTS{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		modelID:    modelID,
		voiceID:    voiceID,
		httpClient: client,
	}
}

// Synthesize converts text into speech audio bytes using ElevenLabs API.
func (e *ElevenLabsTTS) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, "", errors.New("audio: input text is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s/v1/text-to-speech/%s", e.baseURL, e.voiceID)

	reqPayload := map[string]any{
		"text":     trimmed,
		"model_id": e.modelID,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, "", fmt.Errorf("audio: marshaling elevenlabs request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("audio: creating elevenlabs request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")
	if e.apiKey != "" {
		req.Header.Set("xi-api-key", e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("audio: executing elevenlabs request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("audio: elevenlabs tts failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("audio: reading elevenlabs response body: %w", err)
	}

	return audioData, core.MimeTypeMP3, nil
}
