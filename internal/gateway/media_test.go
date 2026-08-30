package gateway_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/gateway"
	"go.uber.org/goleak"
)

func TestSniffContentType(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	gifHeader := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00")
	oggHeader := []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00")
	wavHeader := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00"), make([]byte, 16)...)

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{name: "PNG image", data: pngHeader, expected: "image/png"},
		{name: "JPEG image", data: jpegHeader, expected: "image/jpeg"},
		{name: "GIF image", data: gifHeader, expected: "image/gif"},
		{name: "OGG audio", data: oggHeader, expected: "audio/ogg"},
		{name: "WAV audio", data: wavHeader, expected: "audio/wav"},
		{name: "disallowed executable ELF", data: []byte{0x7F, 'E', 'L', 'F', 1, 1, 1, 0}, expected: ""},
		{name: "disallowed plain text", data: []byte("Hello, this is just plain text without media headers"), expected: ""},
		{name: "empty bytes", data: []byte{}, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gateway.SniffContentType(tt.data)
			if got != tt.expected {
				t.Errorf("SniffContentType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsAllowedMime(t *testing.T) {
	tests := []struct {
		mime      string
		wantImage bool
		wantAudio bool
	}{
		{mime: "image/png", wantImage: true, wantAudio: false},
		{mime: "image/jpeg", wantImage: true, wantAudio: false},
		{mime: "image/webp", wantImage: true, wantAudio: false},
		{mime: "image/gif", wantImage: true, wantAudio: false},
		{mime: "image/svg+xml", wantImage: false, wantAudio: false},
		{mime: "audio/ogg", wantImage: false, wantAudio: true},
		{mime: "audio/wav", wantImage: false, wantAudio: true},
		{mime: "audio/mpeg", wantImage: false, wantAudio: true},
		{mime: "audio/mp4", wantImage: false, wantAudio: true},
		{mime: "audio/x-m4a", wantImage: false, wantAudio: true},
		{mime: "text/plain", wantImage: false, wantAudio: false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			if got := gateway.IsAllowedImageMime(tt.mime); got != tt.wantImage {
				t.Errorf("IsAllowedImageMime(%q) = %v, want %v", tt.mime, got, tt.wantImage)
			}
			if got := gateway.IsAllowedAudioMime(tt.mime); got != tt.wantAudio {
				t.Errorf("IsAllowedAudioMime(%q) = %v, want %v", tt.mime, got, tt.wantAudio)
			}
		})
	}
}

func TestDownloadMedia_Success(t *testing.T) {
	defer goleak.VerifyNone(t)

	pngBytes := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("payload")...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, mime, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 10*1024*1024)
	if err != nil {
		t.Fatalf("DownloadMedia() failed: %v", err)
	}

	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png", mime)
	}
	if string(data) != string(pngBytes) {
		t.Errorf("data mismatch, got %d bytes, want %d bytes", len(data), len(pngBytes))
	}
}

func TestDownloadMedia_ExceedsContentLength(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "20000000") // 20MB
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx := context.Background()
	_, _, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 10*1024*1024)
	if !errors.Is(err, gateway.ErrMediaTooLarge) {
		t.Fatalf("expected ErrMediaTooLarge, got: %v", err)
	}
}

func TestDownloadMedia_ExceedsStreamLimit(t *testing.T) {
	defer goleak.VerifyNone(t)

	largePayload := make([]byte, 1024*1024+100) // 1MB + 100 bytes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not set Content-Length header, stream directly
		_, _ = w.Write(largePayload)
	}))
	defer server.Close()

	ctx := context.Background()
	// limit to 1MB
	_, _, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 1024*1024)
	if !errors.Is(err, gateway.ErrMediaTooLarge) {
		t.Fatalf("expected ErrMediaTooLarge, got: %v", err)
	}
}

func TestDownloadMedia_UnsupportedMime(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte{0x7F, 'E', 'L', 'F', 1, 1, 1, 0, 0, 0})
	}))
	defer server.Close()

	ctx := context.Background()
	_, _, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 10*1024*1024)
	if !errors.Is(err, gateway.ErrUnsupportedMediaType) {
		t.Fatalf("expected ErrUnsupportedMediaType, got: %v", err)
	}
}

func TestDownloadMedia_ContextCanceled(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("too late"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Canceled immediately

	_, _, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 10*1024*1024)
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}

func TestDownloadMedia_Non200Status(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	_, _, err := gateway.DownloadMedia(ctx, server.Client(), server.URL, 10*1024*1024)
	if err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}
