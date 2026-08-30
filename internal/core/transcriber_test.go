package core_test

import (
	"context"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

type mockTranscriber struct {
	transcribedText string
	err             error
}

func (m *mockTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.transcribedText, nil
}

func TestTranscriber_Interface(t *testing.T) {
	var transcriber core.Transcriber = &mockTranscriber{
		transcribedText: "test transcript",
	}

	got, err := transcriber.Transcribe(context.Background(), []byte("fake audio"), "audio/ogg")
	if err != nil {
		t.Fatalf("Transcribe() error = %v, want nil", err)
	}
	if got != "test transcript" {
		t.Errorf("Transcribe() = %q, want %q", got, "test transcript")
	}
}
