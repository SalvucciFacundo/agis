package gateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"go.uber.org/goleak"
)

type mockTranscriber struct {
	mu           sync.Mutex
	calls        int
	lastMimeType string
	lastAudioLen int
	resultText   string
	err          error
}

func (m *mockTranscriber) Transcribe(ctx context.Context, audio []byte, mimeType string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastMimeType = mimeType
	m.lastAudioLen = len(audio)
	if m.err != nil {
		return "", m.err
	}
	return m.resultText, nil
}

func TestTelegramAdapter_PhotoIngestion(t *testing.T) {
	defer goleak.VerifyNone(t)

	pngData := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("telegram_photo")...)

	var getUpdatesCalls int
	var getFileCalls int
	var downloadCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/botTEST_TOKEN/getUpdates":
			getUpdatesCalls++
			if getUpdatesCalls == 1 {
				resp := map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 100,
							"message": map[string]any{
								"message_id": 42,
								"from": map[string]any{
									"id":       12345,
									"username": "tester",
								},
								"chat": map[string]any{
									"id": 999,
								},
								"date":    time.Now().Unix(),
								"caption": "look at this diagram",
								"photo": []map[string]any{
									{
										"file_id":   "small_photo_id",
										"file_size": 100,
										"width":     100,
										"height":    100,
									},
									{
										"file_id":   "large_photo_id",
										"file_size": len(pngData),
										"width":     1024,
										"height":    768,
									},
								},
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})

		case r.URL.Path == "/botTEST_TOKEN/getFile":
			getFileCalls++
			fileID := r.URL.Query().Get("file_id")
			if fileID != "large_photo_id" {
				t.Errorf("getFile called with file_id = %q, want large_photo_id", fileID)
			}
			resp := map[string]any{
				"ok": true,
				"result": map[string]any{
					"file_id":   fileID,
					"file_path": "photos/photo_large.png",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/file/botTEST_TOKEN/photos/photo_large.png":
			downloadCalls++
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

	adapter := gateway.NewTelegramAdapter(
		config.TelegramConfig{
			Enabled:   true,
			Token:     "TEST_TOKEN",
			Allowlist: []string{"12345"},
		},
		gateway.WithTelegramBaseURL(server.URL),
		gateway.WithTelegramHandler(handler),
		gateway.WithTelegramPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Content != "look at this diagram" {
			t.Errorf("ev.Content = %q, want 'look at this diagram'", ev.Content)
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
		if string(att.Data) != string(pngData) {
			t.Errorf("att.Data mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for telegram photo event")
	}

	_ = adapter.Stop()
}

func TestTelegramAdapter_VoiceIngestion_WithTranscriber(t *testing.T) {
	defer goleak.VerifyNone(t)

	oggData := append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"), []byte("sample_voice_bytes")...)
	transcriber := &mockTranscriber{
		resultText: "Please book a meeting for tomorrow at 3 PM",
	}

	var getUpdatesCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/botTEST_TOKEN/getUpdates":
			getUpdatesCalls++
			if getUpdatesCalls == 1 {
				resp := map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 200,
							"message": map[string]any{
								"message_id": 43,
								"from": map[string]any{
									"id":       12345,
									"username": "tester",
								},
								"chat": map[string]any{
									"id": 999,
								},
								"date": time.Now().Unix(),
								"voice": map[string]any{
									"file_id":   "voice_note_id",
									"mime_type": "audio/ogg",
									"file_size": len(oggData),
								},
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})

		case r.URL.Path == "/botTEST_TOKEN/getFile":
			resp := map[string]any{
				"ok": true,
				"result": map[string]any{
					"file_id":   "voice_note_id",
					"file_path": "voice/note.ogg",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/file/botTEST_TOKEN/voice/note.ogg":
			w.Header().Set("Content-Type", "audio/ogg")
			_, _ = w.Write(oggData)

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

	adapter := gateway.NewTelegramAdapter(
		config.TelegramConfig{
			Enabled:   true,
			Token:     "TEST_TOKEN",
			Allowlist: []string{"12345"},
		},
		gateway.WithTelegramBaseURL(server.URL),
		gateway.WithTelegramHandler(handler),
		gateway.WithTelegramTranscriber(transcriber),
		gateway.WithTelegramPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Content != "Please book a meeting for tomorrow at 3 PM" {
			t.Errorf("ev.Content = %q, want transcriber result", ev.Content)
		}
		if len(ev.Attachments) != 1 {
			t.Fatalf("ev.Attachments length = %d, want 1", len(ev.Attachments))
		}
		att := ev.Attachments[0]
		if att.Type != "audio" {
			t.Errorf("att.Type = %q, want 'audio'", att.Type)
		}
		if att.MimeType != "audio/ogg" {
			t.Errorf("att.MimeType = %q, want 'audio/ogg'", att.MimeType)
		}
		if string(att.Data) != string(oggData) {
			t.Errorf("att.Data mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for telegram voice event")
	}

	_ = adapter.Stop()
}
