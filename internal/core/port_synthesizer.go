package core

import "context"

const (
	// Standard MIME types for synthesized audio streams.
	MimeTypeMP3  = "audio/mpeg"
	MimeTypeOpus = "audio/opus"
	MimeTypeOGG  = "audio/ogg"
	MimeTypeAAC  = "audio/aac"
	MimeTypeWAV  = "audio/wav"
)

// Synthesizer defines the outbound port for transforming text into synthesized spoken audio.
type Synthesizer interface {
	// Synthesize converts text into audio bytes and returns the payload alongside its MIME type,
	// or an error if the synthesis request fails.
	Synthesize(ctx context.Context, text string) ([]byte, string, error)
}
