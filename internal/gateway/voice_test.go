package gateway_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
)

type mockSynthesizer struct {
	mu         sync.Mutex
	audio      []byte
	mimeType   string
	err        error
	calledWith string
}

func (m *mockSynthesizer) Synthesize(ctx context.Context, text string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calledWith = text
	if m.err != nil {
		return nil, "", m.err
	}
	return m.audio, m.mimeType, nil
}

type mockVoiceAdapter struct {
	name          string
	sendCalled    bool
	lastSentText  string
	voiceCalled   bool
	lastVoiceData []byte
	lastVoiceMime string
}

func (m *mockVoiceAdapter) Name() string { return m.name }
func (m *mockVoiceAdapter) Start(ctx context.Context) error { return nil }
func (m *mockVoiceAdapter) Stop() error { return nil }
func (m *mockVoiceAdapter) Send(ctx context.Context, target string, msg string) error {
	m.sendCalled = true
	m.lastSentText = msg
	return nil
}
func (m *mockVoiceAdapter) SendVoice(ctx context.Context, target string, audio []byte, mimeType string, caption string) error {
	m.voiceCalled = true
	m.lastVoiceData = audio
	m.lastVoiceMime = mimeType
	return nil
}

var _ gateway.Adapter = (*mockVoiceAdapter)(nil)
var _ gateway.VoiceSender = (*mockVoiceAdapter)(nil)

type mockBrainRunner struct {
	stepCalled bool
}

func (m *mockBrainRunner) Step(ctx context.Context, input string) error {
	m.stepCalled = true
	return nil
}
func (m *mockBrainRunner) SetActiveConversation(id string) {}

type mockRepoWithReply struct {
	core.Repository
	replyText string
}

func (m *mockRepoWithReply) CreateConversation(ctx context.Context, title string) (*core.Conversation, error) {
	return &core.Conversation{ID: "test-conv-1"}, nil
}

func (m *mockRepoWithReply) Messages(ctx context.Context, convID string, limit int) ([]core.Message, error) {
	return []core.Message{
		{Role: core.RoleAssistant, Content: m.replyText},
	}, nil
}

func TestTelegramAdapter_SendVoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottoken123/sendVoice" {
			t.Errorf("expected path /bottoken123/sendVoice, got %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data, got %s", r.Header.Get("Content-Type"))
		}

		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("parsing multipart form: %v", err)
		}

		if r.MultipartForm.Value["chat_id"][0] != "998877" {
			t.Errorf("expected chat_id 998877, got %v", r.MultipartForm.Value["chat_id"])
		}
		fileHeader := r.MultipartForm.File["voice"][0]
		file, _ := fileHeader.Open()
		data, _ := io.ReadAll(file)
		if string(data) != "voice-bytes-payload" {
			t.Errorf("expected voice-bytes-payload, got %q", string(data))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	adapter := gateway.NewTelegramAdapter(config.TelegramConfig{
		Token: "token123",
	}, gateway.WithTelegramBaseURL(server.URL))

	ctx := context.Background()
	err := adapter.SendVoice(ctx, "998877", []byte("voice-bytes-payload"), "audio/ogg", "")
	if err != nil {
		t.Fatalf("SendVoice failed: %v", err)
	}
}

func TestWhatsAppAdapter_SendVoice(t *testing.T) {
	var mediaUploaded, messageSent bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/media") {
			mediaUploaded = true
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Errorf("expected multipart/form-data on media upload, got %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": "meta-media-id-777"}`))
			return
		}

		if strings.HasSuffix(r.URL.Path, "/messages") {
			messageSent = true
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "meta-media-id-777") {
				t.Errorf("expected media ID in body, got %s", string(body))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"messages": [{"id": "wamid.123"}]}`))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := gateway.NewWhatsAppAdapter(config.WhatsAppConfig{
		PhoneNumberID: "phone-123",
		APIToken:      "wa-token",
	}, gateway.WithWhatsAppBaseURL(server.URL))

	ctx := context.Background()
	err := adapter.SendVoice(ctx, "123456789", []byte("wa-audio-bytes"), "audio/ogg", "")
	if err != nil {
		t.Fatalf("SendVoice failed: %v", err)
	}
	if !mediaUploaded {
		t.Errorf("media was not uploaded")
	}
	if !messageSent {
		t.Errorf("message was not sent")
	}
}

func TestMultiplexer_VoiceReplyIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("synthesizes and sends voice when user sent audio", func(t *testing.T) {
		adapter := &mockVoiceAdapter{name: "mock-voice"}
		synth := &mockSynthesizer{
			audio:    []byte("synthesized-audio"),
			mimeType: core.MimeTypeMP3,
		}
		repo := &mockRepoWithReply{replyText: "Spoken assistant response"}
		brain := &mockBrainRunner{}

		mux := gateway.NewMultiplexer(
			gateway.WithMultiplexerBrain(brain),
			gateway.WithMultiplexerRepository(repo),
			gateway.WithMultiplexerSynthesizer(synth),
		)
		mux.RegisterAdapter(adapter)

		ev := gateway.MessageEvent{
			Adapter: "mock-voice",
			ChatID:  "chat-123",
			Content: "voice note transcript",
			Attachments: []core.Attachment{
				{Type: "audio", MimeType: "audio/ogg", Data: []byte("inbound-audio")},
			},
			Timestamp: time.Now(),
		}

		err := mux.HandleEvent(ctx, ev)
		if err != nil {
			t.Fatalf("HandleEvent failed: %v", err)
		}

		if !adapter.voiceCalled {
			t.Errorf("expected SendVoice to be called on adapter")
		}
		if adapter.sendCalled {
			t.Errorf("expected Send (text) not to be called when voice succeeded")
		}
		if string(adapter.lastVoiceData) != "synthesized-audio" {
			t.Errorf("expected audio 'synthesized-audio', got %q", string(adapter.lastVoiceData))
		}
	})

	t.Run("falls back to text when synthesis fails", func(t *testing.T) {
		adapter := &mockVoiceAdapter{name: "mock-voice"}
		synth := &mockSynthesizer{
			err: errors.New("tts rate limit exceeded"),
		}
		repo := &mockRepoWithReply{replyText: "Fallback text response"}
		brain := &mockBrainRunner{}

		mux := gateway.NewMultiplexer(
			gateway.WithMultiplexerBrain(brain),
			gateway.WithMultiplexerRepository(repo),
			gateway.WithMultiplexerSynthesizer(synth),
		)
		mux.RegisterAdapter(adapter)

		ev := gateway.MessageEvent{
			Adapter: "mock-voice",
			ChatID:  "chat-123",
			Content: "voice note transcript",
			Attachments: []core.Attachment{
				{Type: "audio", MimeType: "audio/ogg"},
			},
			Timestamp: time.Now(),
		}

		err := mux.HandleEvent(ctx, ev)
		if err != nil {
			t.Fatalf("HandleEvent failed: %v", err)
		}

		if adapter.voiceCalled {
			t.Errorf("expected SendVoice not to be called on synthesis error")
		}
		if !adapter.sendCalled {
			t.Errorf("expected fallback Send (text) to be called")
		}
		if adapter.lastSentText != "Fallback text response" {
			t.Errorf("expected text 'Fallback text response', got %q", adapter.lastSentText)
		}
	})

	t.Run("sends text when user sent plain text without audio", func(t *testing.T) {
		adapter := &mockVoiceAdapter{name: "mock-voice"}
		synth := &mockSynthesizer{audio: []byte("audio")}
		repo := &mockRepoWithReply{replyText: "Text only reply"}
		brain := &mockBrainRunner{}

		mux := gateway.NewMultiplexer(
			gateway.WithMultiplexerBrain(brain),
			gateway.WithMultiplexerRepository(repo),
			gateway.WithMultiplexerSynthesizer(synth),
		)
		mux.RegisterAdapter(adapter)

		ev := gateway.MessageEvent{
			Adapter:   "mock-voice",
			ChatID:    "chat-123",
			Content:   "plain text message",
			Timestamp: time.Now(),
		}

		err := mux.HandleEvent(ctx, ev)
		if err != nil {
			t.Fatalf("HandleEvent failed: %v", err)
		}

		if adapter.voiceCalled {
			t.Errorf("did not expect voice reply for text input")
		}
		if !adapter.sendCalled {
			t.Errorf("expected Send (text) to be called")
		}
	})
}
