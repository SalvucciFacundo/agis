package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

type mockSynthesizer struct {
	audio    []byte
	mimeType string
	err      error
}

func (m *mockSynthesizer) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if text == "" {
		return nil, "", errors.New("empty text")
	}
	return m.audio, m.mimeType, m.err
}

var _ core.Synthesizer = (*mockSynthesizer)(nil)

func TestSynthesizerMock_Contract(t *testing.T) {
	ctx := context.Background()
	mock := &mockSynthesizer{
		audio:    []byte("fake-mp3-bytes"),
		mimeType: core.MimeTypeMP3,
	}

	t.Run("successful synthesis", func(t *testing.T) {
		audio, mime, err := mock.Synthesize(ctx, "Hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mime != core.MimeTypeMP3 {
			t.Errorf("got mime %q, want %q", mime, core.MimeTypeMP3)
		}
		if string(audio) != "fake-mp3-bytes" {
			t.Errorf("got audio %q, want %q", string(audio), "fake-mp3-bytes")
		}
	})

	t.Run("empty text error", func(t *testing.T) {
		_, _, err := mock.Synthesize(ctx, "")
		if err == nil {
			t.Fatal("expected error for empty text, got nil")
		}
	})

	t.Run("cancelled context error", func(t *testing.T) {
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := mock.Synthesize(canceledCtx, "Hello")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})
}
