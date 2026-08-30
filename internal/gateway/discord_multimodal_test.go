package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"go.uber.org/goleak"
)

func TestDiscordAdapter_ImageAttachmentIngestion(t *testing.T) {
	defer goleak.VerifyNone(t)

	pngData := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("discord_image")...)

	var messagesCalls int
	var cdnCalls int

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/channels/channel-123/messages":
			messagesCalls++
			if messagesCalls == 1 {
				resp := []map[string]any{
					{
						"id":         "msg-999",
						"channel_id": "channel-123",
						"content":    "here is my architecture",
						"author": map[string]any{
							"id":       "user-456",
							"username": "tester",
							"bot":      false,
						},
						"timestamp": time.Now().Format(time.RFC3339),
						"attachments": []map[string]any{
							{
								"id":           "att-1",
								"filename":     "arch.png",
								"content_type": "image/png",
								"size":         len(pngData),
								"url":          server.URL + "/cdn/attachments/arch.png",
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode([]any{})

		case r.URL.Path == "/cdn/attachments/arch.png":
			cdnCalls++
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngData)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	events := make(chan gateway.MessageEvent, 1)
	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		events <- ev
		return nil
	}

	adapter := gateway.NewDiscordAdapter(
		config.DiscordConfig{
			Enabled:   true,
			Token:     "DISCORD_TOKEN",
			Allowlist: []string{"user-456"},
		},
		gateway.WithDiscordBaseURL(server.URL),
		gateway.WithDiscordPollChannels([]string{"channel-123"}),
		gateway.WithDiscordHandler(handler),
		gateway.WithDiscordPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Content != "here is my architecture" {
			t.Errorf("ev.Content = %q, want 'here is my architecture'", ev.Content)
		}
		if len(ev.Attachments) != 1 {
			t.Fatalf("ev.Attachments length = %d, want 1", len(ev.Attachments))
		}
		att := ev.Attachments[0]
		if att.Type != "image" {
			t.Errorf("att.Type = %q, want 'image'", att.Type)
		}
		if att.MimeType != "image/png" {
			t.Errorf("att.MimeType = %q, want 'image/png'", att.MimeType)
		}
		if att.Name != "arch.png" {
			t.Errorf("att.Name = %q, want 'arch.png'", att.Name)
		}
		if string(att.Data) != string(pngData) {
			t.Errorf("att.Data mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for discord image attachment event")
	}

	_ = adapter.Stop()
}

func TestDiscordAdapter_AudioAttachmentIngestion_WithTranscriber(t *testing.T) {
	defer goleak.VerifyNone(t)

	wavData := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00"), make([]byte, 16)...)
	wavData = append(wavData, []byte("data\x04\x00\x00\x001234")...)

	transcriber := &mockTranscriber{
		resultText: "Transcribed voice note from Discord",
	}

	var messagesCalls int
	var server *httptest.Server

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/channels/channel-123/messages":
			messagesCalls++
			if messagesCalls == 1 {
				resp := []map[string]any{
					{
						"id":         "msg-1000",
						"channel_id": "channel-123",
						"content":    "Listen to this recording:",
						"author": map[string]any{
							"id":       "user-456",
							"username": "tester",
							"bot":      false,
						},
						"timestamp": time.Now().Format(time.RFC3339),
						"attachments": []map[string]any{
							{
								"id":           "att-2",
								"filename":     "recording.wav",
								"content_type": "audio/wav",
								"size":         len(wavData),
								"url":          server.URL + "/cdn/attachments/recording.wav",
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode([]any{})

		case r.URL.Path == "/cdn/attachments/recording.wav":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write(wavData)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	events := make(chan gateway.MessageEvent, 1)
	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		events <- ev
		return nil
	}

	adapter := gateway.NewDiscordAdapter(
		config.DiscordConfig{
			Enabled:   true,
			Token:     "DISCORD_TOKEN",
			Allowlist: []string{"user-456"},
		},
		gateway.WithDiscordBaseURL(server.URL),
		gateway.WithDiscordPollChannels([]string{"channel-123"}),
		gateway.WithDiscordHandler(handler),
		gateway.WithDiscordTranscriber(transcriber),
		gateway.WithDiscordPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case ev := <-events:
		expectedContent := "Listen to this recording:\nTranscribed voice note from Discord"
		if ev.Content != expectedContent {
			t.Errorf("ev.Content = %q, want %q", ev.Content, expectedContent)
		}
		if len(ev.Attachments) != 1 {
			t.Fatalf("ev.Attachments length = %d, want 1", len(ev.Attachments))
		}
		att := ev.Attachments[0]
		if att.Type != "audio" {
			t.Errorf("att.Type = %q, want 'audio'", att.Type)
		}
		if att.MimeType != "audio/wav" {
			t.Errorf("att.MimeType = %q, want 'audio/wav'", att.MimeType)
		}
		if string(att.Data) != string(wavData) {
			t.Errorf("att.Data mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for discord audio attachment event")
	}

	_ = adapter.Stop()
}
