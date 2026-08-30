package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"go.uber.org/goleak"
)

func TestWhisper_ImplementsTranscriber(t *testing.T) {
	var _ core.Transcriber = (*Whisper)(nil)
}

func TestWhisper_Transcribe_Success(t *testing.T) {
	defer goleak.VerifyNone(t)

	var (
		gotMethod   string
		gotPath     string
		gotAuth     string
		gotModel    string
		gotFilename string
		gotFileData []byte
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		gotModel = r.FormValue("model")

		file, header, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			gotFilename = header.Filename
			gotFileData, _ = io.ReadAll(file)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text": "Hello, this is a test voice note."}`))
	}))
	defer server.Close()

	whisper := NewWhisper(server.URL, "sk-testkey", "whisper-1")
	audioData := []byte("OGG_AUDIO_BYTES_TEST")

	text, err := whisper.Transcribe(context.Background(), audioData, "audio/ogg")
	if err != nil {
		t.Fatalf("Transcribe() error = %v, want nil", err)
	}

	if text != "Hello, this is a test voice note." {
		t.Errorf("Transcribe() = %q, want %q", text, "Hello, this is a test voice note.")
	}
	if gotMethod != http.MethodPost {
		t.Errorf("HTTP method = %q, want POST", gotMethod)
	}
	if gotPath != "/audio/transcriptions" {
		t.Errorf("HTTP path = %q, want /audio/transcriptions", gotPath)
	}
	if gotAuth != "Bearer sk-testkey" {
		t.Errorf("Authorization header = %q, want Bearer sk-testkey", gotAuth)
	}
	if gotModel != "whisper-1" {
		t.Errorf("model form value = %q, want whisper-1", gotModel)
	}
	if gotFilename != "audio.ogg" {
		t.Errorf("filename = %q, want audio.ogg", gotFilename)
	}
	if string(gotFileData) != "OGG_AUDIO_BYTES_TEST" {
		t.Errorf("file data = %q, want %q", string(gotFileData), "OGG_AUDIO_BYTES_TEST")
	}
}

func TestWhisper_Transcribe_MIMEFilenameDeduction(t *testing.T) {
	tests := []struct {
		mimeType     string
		wantFilename string
	}{
		{mimeType: "audio/ogg", wantFilename: "audio.ogg"},
		{mimeType: "audio/wav", wantFilename: "audio.wav"},
		{mimeType: "audio/mpeg", wantFilename: "audio.mp3"},
		{mimeType: "audio/mp3", wantFilename: "audio.mp3"},
		{mimeType: "audio/mp4", wantFilename: "audio.mp4"},
		{mimeType: "audio/x-m4a", wantFilename: "audio.m4a"},
		{mimeType: "audio/m4a", wantFilename: "audio.m4a"},
		{mimeType: "audio/webm", wantFilename: "audio.webm"},
		{mimeType: "audio/flac", wantFilename: "audio.flac"},
		{mimeType: "application/octet-stream", wantFilename: "audio.bin"},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			var gotFilename string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseMultipartForm(10 << 20)
				_, header, err := r.FormFile("file")
				if err == nil {
					gotFilename = header.Filename
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"text":"ok"}`))
			}))
			defer server.Close()

			whisper := NewWhisper(server.URL, "key", "")
			_, err := whisper.Transcribe(context.Background(), []byte("audio"), tt.mimeType)
			if err != nil {
				t.Fatalf("Transcribe() error = %v", err)
			}
			if gotFilename != tt.wantFilename {
				t.Errorf("filename for MIME %q = %q, want %q", tt.mimeType, gotFilename, tt.wantFilename)
			}
		})
	}
}

func TestWhisper_Transcribe_EmptyAudio(t *testing.T) {
	whisper := NewWhisper("https://api.openai.com/v1", "key", "whisper-1")

	_, err := whisper.Transcribe(context.Background(), nil, "audio/ogg")
	if err == nil {
		t.Fatal("Transcribe(nil) error = nil, want error")
	}

	_, err = whisper.Transcribe(context.Background(), []byte{}, "audio/ogg")
	if err == nil {
		t.Fatal("Transcribe([]byte{}) error = nil, want error")
	}
}

func TestWhisper_Transcribe_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"done"}`))
	}))
	defer server.Close()

	whisper := NewWhisper(server.URL, "key", "whisper-1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before request

	_, err := whisper.Transcribe(ctx, []byte("some audio data"), "audio/ogg")
	if err == nil {
		t.Fatal("Transcribe() with canceled context error = nil, want context error")
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Errorf("Transcribe() error = %v, want to contain %v", err, context.Canceled)
	}
}

func TestWhisper_Transcribe_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErrSub string
	}{
		{
			name:       "401 unauthorized with JSON error",
			statusCode: http.StatusUnauthorized,
			body:       `{"error": {"message": "Incorrect API key provided", "type": "invalid_request_error"}}`,
			wantErrSub: "Incorrect API key provided",
		},
		{
			name:       "400 bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"error": {"message": "Invalid file format", "type": "invalid_request_error"}}`,
			wantErrSub: "Invalid file format",
		},
		{
			name:       "500 internal server error plain text",
			statusCode: http.StatusInternalServerError,
			body:       "Internal Server Error",
			wantErrSub: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			whisper := NewWhisper(server.URL, "key", "whisper-1")
			_, err := whisper.Transcribe(context.Background(), []byte("audio"), "audio/ogg")
			if err == nil {
				t.Fatalf("Transcribe() error = nil, want error containing %q", tt.wantErrSub)
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("Transcribe() error = %v, want to contain %q", err, tt.wantErrSub)
			}
		})
	}
}
