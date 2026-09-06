package gateway_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/gateway"
)

func computeWhatsAppSignature(appSecret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWhatsAppAdapter_Contract(t *testing.T) {
	adapter := gateway.NewWhatsAppAdapter(config.WhatsAppConfig{})
	if adapter.Name() != "whatsapp" {
		t.Errorf("Name() = %q, want 'whatsapp'", adapter.Name())
	}
}

func TestWhatsAppAdapter_WebhookVerification_GET(t *testing.T) {
	verifyToken := "my-secret-verify-token"
	adapter := gateway.NewWhatsAppAdapter(config.WhatsAppConfig{
		VerifyToken: verifyToken,
	})

	tests := []struct {
		name       string
		mode       string
		token      string
		challenge  string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "valid subscription verification",
			mode:       "subscribe",
			token:      verifyToken,
			challenge:  "1158201444",
			wantStatus: http.StatusOK,
			wantBody:   "1158201444",
		},
		{
			name:       "invalid verify token",
			mode:       "subscribe",
			token:      "wrong-token",
			challenge:  "1158201444",
			wantStatus: http.StatusForbidden,
			wantBody:   "",
		},
		{
			name:       "invalid hub.mode",
			mode:       "unsubscribe",
			token:      verifyToken,
			challenge:  "1158201444",
			wantStatus: http.StatusForbidden,
			wantBody:   "",
		},
		{
			name:       "missing hub.mode",
			mode:       "",
			token:      verifyToken,
			challenge:  "1158201444",
			wantStatus: http.StatusForbidden,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/whatsapp/webhook?hub.mode=%s&hub.verify_token=%s&hub.challenge=%s",
				tt.mode, tt.token, tt.challenge)
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			adapter.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestWhatsAppAdapter_SignatureVerification_POST(t *testing.T) {
	appSecret := "meta-app-secret-12345"
	adapter := gateway.NewWhatsAppAdapter(config.WhatsAppConfig{
		AppSecret:    appSecret,
		AllowedUsers: []string{"+15551234567"},
	})

	body := []byte(`{
		"object": "whatsapp_business_account",
		"entry": [{
			"id": "123",
			"changes": [{
				"value": {
					"messaging_product": "whatsapp",
					"metadata": {"display_phone_number": "15550000000", "phone_number_id": "phone123"},
					"messages": [{
						"from": "+15551234567",
						"id": "wamid.123",
						"timestamp": "1600000000",
						"type": "text",
						"text": {"body": "ping"}
					}]
				},
				"field": "messages"
			}]
		}]
	}`)

	tests := []struct {
		name       string
		sigHeader  string
		wantStatus int
	}{
		{
			name:       "valid signature",
			sigHeader:  computeWhatsAppSignature(appSecret, body),
			wantStatus: http.StatusOK,
		},
		{
			name:       "forged signature",
			sigHeader:  computeWhatsAppSignature("wrong-secret", body),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing signature header",
			sigHeader:  "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed prefix",
			sigHeader:  "sha1=" + hex.EncodeToString([]byte("something")),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.sigHeader != "" {
				req.Header.Set("X-Hub-Signature-256", tt.sigHeader)
			}
			rec := httptest.NewRecorder()

			adapter.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestWhatsAppAdapter_InboundMessage_Text(t *testing.T) {
	appSecret := "secret-xyz"
	var receivedEvent gateway.MessageEvent
	var eventMu sync.Mutex
	eventReceived := make(chan struct{}, 1)

	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		eventMu.Lock()
		receivedEvent = ev
		eventMu.Unlock()
		select {
		case eventReceived <- struct{}{}:
		default:
		}
		return nil
	}

	adapter := gateway.NewWhatsAppAdapter(
		config.WhatsAppConfig{
			AppSecret:    appSecret,
			AllowedUsers: []string{"+15551234567"},
		},
		gateway.WithWhatsAppHandler(handler),
	)

	t.Run("authorized sender dispatches MessageEvent", func(t *testing.T) {
		body := []byte(`{
			"object": "whatsapp_business_account",
			"entry": [{
				"id": "123",
				"changes": [{
					"value": {
						"messaging_product": "whatsapp",
						"metadata": {"phone_number_id": "phone123"},
						"messages": [{
							"from": "+15551234567",
							"id": "wamid.ABC",
							"timestamp": "1700000000",
							"type": "text",
							"text": {"body": "Hello AGIS WhatsApp!"}
						}]
					},
					"field": "messages"
				}]
			}]
		}`)

		req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature(appSecret, body))
		rec := httptest.NewRecorder()

		adapter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		select {
		case <-eventReceived:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for event handler")
		}

		eventMu.Lock()
		defer eventMu.Unlock()
		if receivedEvent.Adapter != "whatsapp" {
			t.Errorf("Adapter = %q, want 'whatsapp'", receivedEvent.Adapter)
		}
		if receivedEvent.UserID != "+15551234567" {
			t.Errorf("UserID = %q, want '+15551234567'", receivedEvent.UserID)
		}
		if receivedEvent.ChatID != "+15551234567" {
			t.Errorf("ChatID = %q, want '+15551234567'", receivedEvent.ChatID)
		}
		if receivedEvent.Content != "Hello AGIS WhatsApp!" {
			t.Errorf("Content = %q, want 'Hello AGIS WhatsApp!'", receivedEvent.Content)
		}
	})

	t.Run("unauthorized sender dropped", func(t *testing.T) {
		body := []byte(`{
			"object": "whatsapp_business_account",
			"entry": [{
				"id": "123",
				"changes": [{
					"value": {
						"messaging_product": "whatsapp",
						"metadata": {"phone_number_id": "phone123"},
						"messages": [{
							"from": "+19999999999",
							"id": "wamid.UNAUTH",
							"timestamp": "1700000000",
							"type": "text",
							"text": {"body": "I am unauthorized"}
						}]
					},
					"field": "messages"
				}]
			}]
		}`)

		req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature(appSecret, body))
		rec := httptest.NewRecorder()

		adapter.ServeHTTP(rec, req)

		// Meta expects 200 OK so it doesn't repeatedly retry dropped events
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		select {
		case <-eventReceived:
			t.Fatal("handler should not be called for unauthorized sender")
		case <-time.After(100 * time.Millisecond):
			// Success
		}
	})
}

type mockWhatsAppTranscriber struct {
	transcribeFn func(ctx context.Context, audioData []byte, mimeType string) (string, error)
}

func (m *mockWhatsAppTranscriber) Transcribe(ctx context.Context, audioData []byte, mimeType string) (string, error) {
	if m.transcribeFn != nil {
		return m.transcribeFn(ctx, audioData, mimeType)
	}
	return "mock transcription", nil
}

func TestWhatsAppAdapter_InboundMessage_VoiceAndAudio(t *testing.T) {
	appSecret := "secret-audio"
	apiToken := "meta-api-token-test"

	var receivedEvent gateway.MessageEvent
	var eventMu sync.Mutex
	eventReceived := make(chan struct{}, 1)

	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		eventMu.Lock()
		receivedEvent = ev
		eventMu.Unlock()
		select {
		case eventReceived <- struct{}{}:
		default:
		}
		return nil
	}

	rawAudioBytes := []byte("OggS\x00\x02mock_whatsapp_ogg_voice_note_payload")

	// Mock Graph API server for media URL lookup and download
	var graphServer *httptest.Server
	graphServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+apiToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case strings.HasPrefix(r.URL.Path, "/media_123"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"url":       graphServer.URL + "/download/media_123.ogg",
				"mime_type": "audio/ogg",
				"file_size": len(rawAudioBytes),
				"id":        "media_123",
			})
		case strings.HasPrefix(r.URL.Path, "/download/"):
			w.Header().Set("Content-Type", "audio/ogg")
			_, _ = w.Write(rawAudioBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer graphServer.Close()

	transcriber := &mockWhatsAppTranscriber{
		transcribeFn: func(ctx context.Context, audioData []byte, mimeType string) (string, error) {
			if !bytes.Equal(audioData, rawAudioBytes) {
				return "", fmt.Errorf("unexpected audio data")
			}
			return "Voice note transcribed successfully", nil
		},
	}

	adapter := gateway.NewWhatsAppAdapter(
		config.WhatsAppConfig{
			APIToken:     apiToken,
			AppSecret:    appSecret,
			AllowedUsers: []string{"+15551234567"},
		},
		gateway.WithWhatsAppBaseURL(graphServer.URL),
		gateway.WithWhatsAppHandler(handler),
		gateway.WithWhatsAppTranscriber(transcriber),
	)

	body := []byte(`{
		"object": "whatsapp_business_account",
		"entry": [{
			"id": "123",
			"changes": [{
				"value": {
					"messaging_product": "whatsapp",
					"metadata": {"phone_number_id": "phone123"},
					"messages": [{
						"from": "+15551234567",
						"id": "wamid.VOICE",
						"timestamp": "1700000000",
						"type": "voice",
						"voice": {
							"id": "media_123",
							"mime_type": "audio/ogg; codecs=opus"
						}
					}]
				},
				"field": "messages"
			}]
		}]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature(appSecret, body))
	rec := httptest.NewRecorder()

	adapter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case <-eventReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for voice note handler")
	}

	eventMu.Lock()
	defer eventMu.Unlock()
	if receivedEvent.Content != "Voice note transcribed successfully" {
		t.Errorf("Content = %q, want 'Voice note transcribed successfully'", receivedEvent.Content)
	}
	if len(receivedEvent.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(receivedEvent.Attachments))
	}
	if receivedEvent.Attachments[0].Type != "audio" {
		t.Errorf("Attachment Type = %q, want 'audio'", receivedEvent.Attachments[0].Type)
	}
}

func TestWhatsAppAdapter_Send_OutboundAndChunking(t *testing.T) {
	apiToken := "meta-token-xyz"
	phoneNumberID := "10987654321"

	var sentRequests []map[string]any
	var reqMu sync.Mutex

	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+apiToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}
		expectedPath := fmt.Sprintf("/%s/messages", phoneNumberID)
		if r.URL.Path != expectedPath {
			http.Error(w, fmt.Sprintf("invalid path: %s", r.URL.Path), http.StatusNotFound)
			return
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		reqMu.Lock()
		sentRequests = append(sentRequests, payload)
		reqMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messaging_product": "whatsapp",
			"contacts":          []map[string]string{{"input": "+15551234567", "wa_id": "15551234567"}},
			"messages":          []map[string]string{{"id": "wamid.OUT123"}},
		})
	}))
	defer graphServer.Close()

	adapter := gateway.NewWhatsAppAdapter(
		config.WhatsAppConfig{
			APIToken:      apiToken,
			PhoneNumberID: phoneNumberID,
		},
		gateway.WithWhatsAppBaseURL(graphServer.URL),
	)

	t.Run("single message delivery", func(t *testing.T) {
		reqMu.Lock()
		sentRequests = nil
		reqMu.Unlock()

		err := adapter.Send(context.Background(), "+15551234567", "Hello from AGIS")
		if err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		reqMu.Lock()
		defer reqMu.Unlock()
		if len(sentRequests) != 1 {
			t.Fatalf("sentRequests count = %d, want 1", len(sentRequests))
		}
		if sentRequests[0]["to"] != "+15551234567" {
			t.Errorf("to = %v, want '+15551234567'", sentRequests[0]["to"])
		}
		textObj, ok := sentRequests[0]["text"].(map[string]any)
		if !ok || textObj["body"] != "Hello from AGIS" {
			t.Errorf("text.body = %v, want 'Hello from AGIS'", textObj["body"])
		}
	})

	t.Run("message chunking > 4096 runes", func(t *testing.T) {
		reqMu.Lock()
		sentRequests = nil
		reqMu.Unlock()

		longMsg := strings.Repeat("A", 4096) + strings.Repeat("B", 1000)
		err := adapter.Send(context.Background(), "+15551234567", longMsg)
		if err != nil {
			t.Fatalf("Send long msg failed: %v", err)
		}

		reqMu.Lock()
		defer reqMu.Unlock()
		if len(sentRequests) != 2 {
			t.Fatalf("sentRequests count = %d, want 2 chunks", len(sentRequests))
		}
		chunk1 := sentRequests[0]["text"].(map[string]any)["body"].(string)
		chunk2 := sentRequests[1]["text"].(map[string]any)["body"].(string)
		if len([]rune(chunk1)) != 4096 {
			t.Errorf("chunk1 len = %d, want 4096", len([]rune(chunk1)))
		}
		if len([]rune(chunk2)) != 1000 {
			t.Errorf("chunk2 len = %d, want 1000", len([]rune(chunk2)))
		}
	})

	t.Run("empty target returns error", func(t *testing.T) {
		err := adapter.Send(context.Background(), "", "Hello")
		if err == nil {
			t.Errorf("Send with empty target should return error")
		}
	})
}

func TestWhatsAppAdapter_MediaDownload_Errors(t *testing.T) {
	appSecret := "secret-err"
	apiToken := "meta-api-err"

	graphServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/media_toobig"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"url":       "http://" + r.Host + "/download_toobig",
				"mime_type": "audio/ogg",
				"file_size": 2000,
				"id":        "media_toobig",
			})
		case strings.HasPrefix(r.URL.Path, "/download_toobig"):
			w.Header().Set("Content-Type", "audio/ogg")
			_, _ = w.Write(bytes.Repeat([]byte("X"), 2000))
		case strings.HasPrefix(r.URL.Path, "/media_err"):
			http.Error(w, "server error", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer graphServer.Close()

	handlerCalled := false
	handler := func(ctx context.Context, ev gateway.MessageEvent) error {
		handlerCalled = true
		return nil
	}

	adapter := gateway.NewWhatsAppAdapter(
		config.WhatsAppConfig{
			APIToken:     apiToken,
			AppSecret:    appSecret,
			AllowedUsers: []string{"+15551234567"},
		},
		gateway.WithWhatsAppBaseURL(graphServer.URL),
		gateway.WithWhatsAppHandler(handler),
		gateway.WithWhatsAppMaxAudioSize(100), // Max 100 bytes
	)

	t.Run("media too big is rejected and not dispatched", func(t *testing.T) {
		handlerCalled = false
		body := []byte(`{
			"object": "whatsapp_business_account",
			"entry": [{
				"id": "123",
				"changes": [{
					"value": {
						"messaging_product": "whatsapp",
						"metadata": {"phone_number_id": "phone123"},
						"messages": [{
							"from": "+15551234567",
							"id": "wamid.BIG",
							"timestamp": "1700000000",
							"type": "audio",
							"audio": {"id": "media_toobig", "mime_type": "audio/ogg"}
						}]
					},
					"field": "messages"
				}]
			}]
		}`)

		req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature(appSecret, body))
		rec := httptest.NewRecorder()

		adapter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if handlerCalled {
			t.Errorf("handler should not be called when media exceeds size limit")
		}
	})

	t.Run("media metadata lookup failure is handled gracefully", func(t *testing.T) {
		handlerCalled = false
		body := []byte(`{
			"object": "whatsapp_business_account",
			"entry": [{
				"id": "123",
				"changes": [{
					"value": {
						"messaging_product": "whatsapp",
						"metadata": {"phone_number_id": "phone123"},
						"messages": [{
							"from": "+15551234567",
							"id": "wamid.ERR",
							"timestamp": "1700000000",
							"type": "audio",
							"audio": {"id": "media_err", "mime_type": "audio/ogg"}
						}]
					},
					"field": "messages"
				}]
			}]
		}`)

		req := httptest.NewRequest(http.MethodPost, "/whatsapp/webhook", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature(appSecret, body))
		rec := httptest.NewRecorder()

		adapter.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if handlerCalled {
			t.Errorf("handler should not be called when media lookup fails")
		}
	})
}

func TestWhatsAppAdapter_StartAndStop_Lifecycle(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	adapter := gateway.NewWhatsAppAdapter(config.WhatsAppConfig{
		ListenAddr: addr,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := adapter.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
