package main_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"go.uber.org/goleak"
)

type recordingProvider struct {
	mu           sync.Mutex
	requests     []core.ChatRequest
	replyMessage string
}

func (p *recordingProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	return core.ChatResponse{
		Content: p.replyMessage,
	}, nil
}

func (p *recordingProvider) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	reply := p.replyMessage
	p.mu.Unlock()

	ch := make(chan core.StreamEvent, 2)
	go func() {
		defer close(ch)
		ch <- core.StreamEvent{Text: reply}
	}()
	return ch, nil
}

func (p *recordingProvider) Models() []core.ModelInfo {
	return nil
}

func TestMultimodalIntegration_TelegramPhotoToVisionBrain(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test_multimodal_tg.db")
	repo, err := memory.NewRepository(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	defer repo.Close()

	provider := &recordingProvider{
		replyMessage: "I see a green chart with an upward trend.",
	}

	brain := core.NewBrain(repo, provider)

	pngBytes := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("chart_data")...)

	var tgUpdatesCalls int
	var sentReplies []string
	var repliesMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/botTEST_TG/getUpdates":
			tgUpdatesCalls++
			if tgUpdatesCalls == 1 {
				resp := map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 1,
							"message": map[string]any{
								"message_id": 101,
								"from":       map[string]any{"id": 777, "username": "trader"},
								"chat":       map[string]any{"id": 555},
								"date":       time.Now().Unix(),
								"caption":    "Explain this graph",
								"photo": []map[string]any{
									{
										"file_id":   "photo_file_1",
										"file_size": len(pngBytes),
										"width":     800,
										"height":    600,
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

		case r.URL.Path == "/botTEST_TG/getFile":
			resp := map[string]any{
				"ok": true,
				"result": map[string]any{
					"file_id":   "photo_file_1",
					"file_path": "photos/graph.png",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/file/botTEST_TG/photos/graph.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)

		case r.URL.Path == "/botTEST_TG/sendMessage":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			repliesMu.Lock()
			if text, ok := payload["text"].(string); ok {
				sentReplies = append(sentReplies, text)
			}
			repliesMu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 102}})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
	)

	tg := gateway.NewTelegramAdapter(
		config.TelegramConfig{
			Enabled:   true,
			Token:     "TEST_TG",
			Allowlist: []string{"777"},
		},
		gateway.WithTelegramBaseURL(server.URL),
		gateway.WithTelegramHandler(mux.HandleEvent),
		gateway.WithTelegramPollInterval(10*time.Millisecond),
	)
	mux.RegisterAdapter(tg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mux.Start(ctx); err != nil {
		t.Fatalf("mux.Start failed: %v", err)
	}

	// Wait for turn processing
	deadline := time.Now().Add(3 * time.Second)
	var succeeded bool
	for time.Now().Before(deadline) {
		repliesMu.Lock()
		count := len(sentReplies)
		repliesMu.Unlock()
		if count > 0 {
			succeeded = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !succeeded {
		t.Fatal("timed out waiting for assistant reply from Telegram photo ingestion")
	}

	_ = mux.Stop()

	// Verify persistence in SQLite repository
	conv, err := repo.LatestConversation(context.Background())
	if err != nil || conv == nil {
		t.Fatalf("expected conversation created in repo, got %v", err)
	}

	msgs, err := repo.Messages(context.Background(), conv.ID, 10)
	if err != nil || len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages in repo, got %d", len(msgs))
	}

	userMsg := msgs[0]
	if userMsg.Role != core.RoleUser {
		t.Errorf("msgs[0].Role = %q, want user", userMsg.Role)
	}
	if userMsg.Content != "Explain this graph" {
		t.Errorf("userMsg.Content = %q, want 'Explain this graph'", userMsg.Content)
	}
	if len(userMsg.Attachments) != 1 {
		t.Fatalf("userMsg.Attachments len = %d, want 1", len(userMsg.Attachments))
	}
	if userMsg.Attachments[0].MimeType != "image/png" {
		t.Errorf("attachment mime = %q, want image/png", userMsg.Attachments[0].MimeType)
	}

	// Verify request sent to provider
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.requests) == 0 {
		t.Fatal("expected provider to receive requests")
	}
}

func TestMultimodalIntegration_TelegramVoiceToWhisperAndBrain(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test_multimodal_voice.db")
	repo, err := memory.NewRepository(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	defer repo.Close()

	provider := &recordingProvider{
		replyMessage: "Task added to your todo list.",
	}
	brain := core.NewBrain(repo, provider)

	oggBytes := append([]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00"), []byte("voice_memo")...)

	// Mock Whisper API server
	var whisperCalls int
	whisperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/audio/transcriptions" || r.URL.Path == "/v1/audio/transcriptions" {
			whisperCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"text": "Remind me to buy groceries tomorrow",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer whisperServer.Close()

	transcriber := llm.NewWhisper(whisperServer.URL, "TEST_API_KEY", "whisper-1")

	var tgUpdatesCalls int
	var sentReplies []string
	var repliesMu sync.Mutex

	tgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/botTEST_TG/getUpdates":
			tgUpdatesCalls++
			if tgUpdatesCalls == 1 {
				resp := map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 1,
							"message": map[string]any{
								"message_id": 201,
								"from":       map[string]any{"id": 888, "username": "speaker"},
								"chat":       map[string]any{"id": 666},
								"date":       time.Now().Unix(),
								"voice": map[string]any{
									"file_id":   "voice_1",
									"mime_type": "audio/ogg",
									"file_size": len(oggBytes),
								},
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{}})

		case r.URL.Path == "/botTEST_TG/getFile":
			resp := map[string]any{
				"ok": true,
				"result": map[string]any{
					"file_id":   "voice_1",
					"file_path": "voice/memo.ogg",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/file/botTEST_TG/voice/memo.ogg":
			w.Header().Set("Content-Type", "audio/ogg")
			_, _ = w.Write(oggBytes)

		case r.URL.Path == "/botTEST_TG/sendMessage":
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			repliesMu.Lock()
			if text, ok := payload["text"].(string); ok {
				sentReplies = append(sentReplies, text)
			}
			repliesMu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 202}})

		default:
			http.NotFound(w, r)
		}
	}))
	defer tgServer.Close()

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
	)

	tg := gateway.NewTelegramAdapter(
		config.TelegramConfig{
			Enabled:   true,
			Token:     "TEST_TG",
			Allowlist: []string{"888"},
		},
		gateway.WithTelegramBaseURL(tgServer.URL),
		gateway.WithTelegramHandler(mux.HandleEvent),
		gateway.WithTelegramTranscriber(transcriber),
		gateway.WithTelegramPollInterval(10*time.Millisecond),
	)
	mux.RegisterAdapter(tg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mux.Start(ctx); err != nil {
		t.Fatalf("mux.Start failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var succeeded bool
	for time.Now().Before(deadline) {
		repliesMu.Lock()
		count := len(sentReplies)
		repliesMu.Unlock()
		if count > 0 {
			succeeded = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !succeeded {
		t.Fatal("timed out waiting for assistant reply from Telegram voice ingestion")
	}

	_ = mux.Stop()

	if whisperCalls == 0 {
		t.Errorf("expected Whisper transcriber to be called")
	}

	conv, err := repo.LatestConversation(context.Background())
	if err != nil || conv == nil {
		t.Fatalf("expected conversation created in repo")
	}
	msgs, err := repo.Messages(context.Background(), conv.ID, 10)
	if err != nil || len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages in repo")
	}
	if msgs[0].Content != "Remind me to buy groceries tomorrow" {
		t.Errorf("msgs[0].Content = %q, want transcribed text", msgs[0].Content)
	}
}

func TestMultimodalIntegration_DiscordImageAttachment(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test_multimodal_dc.db")
	repo, err := memory.NewRepository(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("NewRepository failed: %v", err)
	}
	defer repo.Close()

	provider := &recordingProvider{
		replyMessage: "That looks like an ER diagram.",
	}
	brain := core.NewBrain(repo, provider)

	pngBytes := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("discord_er_diagram")...)

	var dcMessagesCalls int
	var sentReplies []string
	var repliesMu sync.Mutex

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/channels/dc-channel-1/messages":
			if r.Method == http.MethodGet {
				dcMessagesCalls++
				if dcMessagesCalls == 1 {
					resp := []map[string]any{
						{
							"id":         "dc-msg-1",
							"channel_id": "dc-channel-1",
							"content":    "Review my DB design",
							"author": map[string]any{
								"id":       "dc-user-99",
								"username": "architect",
								"bot":      false,
							},
							"timestamp": time.Now().Format(time.RFC3339),
							"attachments": []map[string]any{
								{
									"id":           "att-dc-1",
									"filename":     "schema.png",
									"content_type": "image/png",
									"size":         len(pngBytes),
									"url":          server.URL + "/cdn/schema.png",
								},
							},
						},
					}
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				_ = json.NewEncoder(w).Encode([]any{})
				return
			}
			if r.Method == http.MethodPost {
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				repliesMu.Lock()
				if text, ok := payload["content"].(string); ok {
					sentReplies = append(sentReplies, text)
				}
				repliesMu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "dc-reply-1"})
				return
			}

		case r.URL.Path == "/cdn/schema.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
	)

	dc := gateway.NewDiscordAdapter(
		config.DiscordConfig{
			Enabled:   true,
			Token:     "TEST_DC",
			Allowlist: []string{"dc-user-99"},
		},
		gateway.WithDiscordBaseURL(server.URL),
		gateway.WithDiscordPollChannels([]string{"dc-channel-1"}),
		gateway.WithDiscordHandler(mux.HandleEvent),
		gateway.WithDiscordPollInterval(10*time.Millisecond),
	)
	mux.RegisterAdapter(dc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mux.Start(ctx); err != nil {
		t.Fatalf("mux.Start failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var succeeded bool
	for time.Now().Before(deadline) {
		repliesMu.Lock()
		count := len(sentReplies)
		repliesMu.Unlock()
		if count > 0 {
			succeeded = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !succeeded {
		t.Fatal("timed out waiting for assistant reply from Discord image ingestion")
	}

	_ = mux.Stop()

	conv, err := repo.LatestConversation(context.Background())
	if err != nil || conv == nil {
		t.Fatalf("expected conversation created in repo")
	}
	msgs, err := repo.Messages(context.Background(), conv.ID, 10)
	if err != nil || len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages in repo")
	}
	if msgs[0].Content != "Review my DB design" {
		t.Errorf("msgs[0].Content = %q, want 'Review my DB design'", msgs[0].Content)
	}
	if len(msgs[0].Attachments) != 1 {
		t.Fatalf("msgs[0].Attachments len = %d, want 1", len(msgs[0].Attachments))
	}
	if msgs[0].Attachments[0].MimeType != "image/png" {
		t.Errorf("attachment mime = %q, want image/png", msgs[0].Attachments[0].MimeType)
	}
}
