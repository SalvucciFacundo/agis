package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

const defaultWhisperModel = "whisper-1"

// Whisper is the OpenAI Whisper audio transcription adapter implementing core.Transcriber.
type Whisper struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

var _ core.Transcriber = (*Whisper)(nil)

// NewWhisper creates a new Whisper transcription adapter targeting baseURL.
// If baseURL is empty, it defaults to the OpenAI v1 API base endpoint.
// If model is empty, it defaults to "whisper-1".
func NewWhisper(baseURL, apiKey, model string) *Whisper {
	if baseURL == "" {
		baseURL = openAIBaseURL
	}
	if model == "" {
		model = defaultWhisperModel
	}
	return &Whisper{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// Transcribe sends a multipart/form-data POST request to /audio/transcriptions
// with the provided audio bytes and returns the transcribed text.
func (w *Whisper) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	if len(audio) == 0 {
		return "", errors.New("audio payload is empty")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	filename := deduceFilename(mimeType)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", w.model); err != nil {
		return "", fmt.Errorf("writing model field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("writing audio payload: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		w.baseURL+"/audio/transcriptions",
		&body,
	)
	if err != nil {
		return "", fmt.Errorf("building transcription request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if w.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("posting transcription request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", httpStatusError(resp)
	}

	var result struct {
		Text  string    `json:"text"`
		Error *apiError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding transcription response: %w", err)
	}
	if result.Error != nil {
		return "", result.Error
	}

	return result.Text, nil
}

// deduceFilename infers an appropriate audio filename with extension from a MIME type.
func deduceFilename(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch mimeType {
	case "audio/ogg", "audio/opus", "application/ogg":
		return "audio.ogg"
	case "audio/wav", "audio/x-wav":
		return "audio.wav"
	case "audio/mpeg", "audio/mp3":
		return "audio.mp3"
	case "audio/mp4":
		return "audio.mp4"
	case "audio/x-m4a", "audio/m4a":
		return "audio.m4a"
	case "audio/webm":
		return "audio.webm"
	case "audio/flac", "audio/x-flac":
		return "audio.flac"
	default:
		return "audio.bin"
	}
}
