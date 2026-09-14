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
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "tts-1"
	defaultOpenAIVoice   = "alloy"
	defaultOpenAIFormat  = "mp3"
)

// OpenAITTSOptions provides configuration for the OpenAI TTS adapter.
type OpenAITTSOptions struct {
	BaseURL    string
	APIKey     string
	Model      string
	Voice      string
	Format     string
	Speed      float64
	HTTPClient *http.Client
}

// OpenAITTS implements core.Synthesizer using the OpenAI /audio/speech API.
type OpenAITTS struct {
	baseURL    string
	apiKey     string
	model      string
	voice      string
	format     string
	speed      float64
	httpClient *http.Client
}

var _ core.Synthesizer = (*OpenAITTS)(nil)

// NewOpenAITTS constructs a new OpenAITTS synthesizer.
func NewOpenAITTS(opts OpenAITTSOptions) *OpenAITTS {
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	model := opts.Model
	if model == "" {
		model = defaultOpenAIModel
	}
	voice := opts.Voice
	if voice == "" {
		voice = defaultOpenAIVoice
	}
	format := opts.Format
	if format == "" {
		format = defaultOpenAIFormat
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &OpenAITTS{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		model:      model,
		voice:      voice,
		format:     format,
		speed:      speed,
		httpClient: client,
	}
}

// Synthesize converts text into speech audio bytes.
func (o *OpenAITTS) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, "", errors.New("audio: input text is empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s/audio/speech", o.baseURL)

	reqPayload := map[string]any{
		"model":           o.model,
		"input":           trimmed,
		"voice":           o.voice,
		"response_format": o.format,
		"speed":           o.speed,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, "", fmt.Errorf("audio: marshaling openai tts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("audio: creating openai tts request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("audio: executing openai tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("audio: openai tts failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("audio: reading openai tts response body: %w", err)
	}

	mimeType := formatToMime(o.format)
	return audioData, mimeType, nil
}

func formatToMime(format string) string {
	switch strings.ToLower(format) {
	case "opus":
		return core.MimeTypeOpus
	case "aac":
		return core.MimeTypeAAC
	case "flac":
		return "audio/flac"
	case "wav":
		return core.MimeTypeWAV
	case "mp3":
		return core.MimeTypeMP3
	default:
		return core.MimeTypeMP3
	}
}
