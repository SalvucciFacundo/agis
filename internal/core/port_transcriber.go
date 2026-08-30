package core

import "context"

// Transcriber defines the port for converting speech/audio media into textual transcripts.
type Transcriber interface {
	// Transcribe processes raw audio payload of the given MIME type (e.g., "audio/ogg", "audio/wav")
	// and returns the transcribed text string or an error if transcription fails.
	Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error)
}
