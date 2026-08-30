// Package gateway provides external chat platform adapters (Telegram, Discord),
// a multiplexer for routing messages between chat platforms and the AGIS brain,
// and media ingestion helpers with security guardrails.
package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DefaultMaxImageSize is the default maximum allowed size for images (10MB).
	DefaultMaxImageSize int64 = 10 * 1024 * 1024

	// DefaultMaxAudioSize is the default maximum allowed size for audio files (25MB).
	DefaultMaxAudioSize int64 = 25 * 1024 * 1024
)

var (
	// ErrMediaTooLarge is returned when media payload exceeds configured size limit.
	ErrMediaTooLarge = errors.New("media exceeds maximum allowed size")

	// ErrUnsupportedMediaType is returned when media MIME type is not allowed.
	ErrUnsupportedMediaType = errors.New("unsupported or disallowed media type")

	// ErrEmptyMedia is returned when media response body is empty.
	ErrEmptyMedia = errors.New("empty media payload")
)

// Allowed image MIME types.
var allowedImageMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// Allowed audio MIME types.
var allowedAudioMimes = map[string]bool{
	"audio/ogg":   true,
	"audio/wav":   true,
	"audio/wave":  true,
	"audio/x-wav": true,
	"audio/mpeg":  true,
	"audio/mp3":   true,
	"audio/mp4":   true,
	"audio/x-m4a": true,
	"audio/aac":   true,
}

// IsAllowedImageMime reports whether the given MIME type is a supported image format.
func IsAllowedImageMime(mime string) bool {
	clean := strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(clean, ";"); idx != -1 {
		clean = clean[:idx]
	}
	return allowedImageMimes[clean]
}

// IsAllowedAudioMime reports whether the given MIME type is a supported audio format.
func IsAllowedAudioMime(mime string) bool {
	clean := strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(clean, ";"); idx != -1 {
		clean = clean[:idx]
	}
	return allowedAudioMimes[clean]
}

// SniffContentType detects the MIME type of data and returns the canonical allowed MIME string.
// If the content is not an allowed image or audio format, it returns an empty string.
func SniffContentType(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// First 512 bytes are sufficient for MIME sniffing
	sniffLen := 512
	if len(data) < sniffLen {
		sniffLen = len(data)
	}
	sample := data[:sniffLen]

	// Custom magic header detections for formats http.DetectContentType may classify as octet-stream
	if bytes.HasPrefix(sample, []byte("OggS")) {
		return "audio/ogg"
	}
	if bytes.HasPrefix(sample, []byte("RIFF")) && len(sample) >= 12 && string(sample[8:12]) == "WAVE" {
		return "audio/wav"
	}

	detected := http.DetectContentType(sample)
	if idx := strings.Index(detected, ";"); idx != -1 {
		detected = detected[:idx]
	}
	detected = strings.ToLower(strings.TrimSpace(detected))

	// Normalize WAV variants
	if detected == "audio/x-wav" || detected == "audio/wave" {
		return "audio/wav"
	}

	if IsAllowedImageMime(detected) || IsAllowedAudioMime(detected) {
		return detected
	}

	return ""
}

// DownloadMedia downloads binary media from a URL using the provided HTTP client with bounded size and MIME guardrails.
func DownloadMedia(ctx context.Context, client *http.Client, url string, maxBytes int64) ([]byte, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxAudioSize
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing media download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	if resp.ContentLength > maxBytes {
		return nil, "", fmt.Errorf("%w: content-length %d exceeds limit %d", ErrMediaTooLarge, resp.ContentLength, maxBytes)
	}

	// Read up to maxBytes + 1 to detect stream overflow
	limitedReader := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", fmt.Errorf("reading media payload: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("%w: received %d bytes exceeding limit %d", ErrMediaTooLarge, len(data), maxBytes)
	}

	if len(data) == 0 {
		return nil, "", ErrEmptyMedia
	}

	mime := SniffContentType(data)
	if mime == "" {
		// Try Content-Type header as fallback if sniffing was ambiguous
		headerType := resp.Header.Get("Content-Type")
		if IsAllowedImageMime(headerType) || IsAllowedAudioMime(headerType) {
			mime = headerType
		} else {
			return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedMediaType, http.DetectContentType(data))
		}
	}

	return data, mime, nil
}
