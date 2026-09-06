package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// WhatsAppMaxMessageLength is WhatsApp's per-message character limit (4096 runes).
const WhatsAppMaxMessageLength = 4096

// WhatsAppConfig carries WhatsApp adapter configuration.
type WhatsAppConfig = config.WhatsAppConfig

// WhatsAppOption configures a WhatsAppAdapter.
type WhatsAppOption func(*WhatsAppAdapter)

// WithWhatsAppBaseURL sets a custom Meta Graph API base URL (useful in tests).
func WithWhatsAppBaseURL(url string) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		a.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithWhatsAppHandler sets the message event handler.
func WithWhatsAppHandler(h Handler) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		a.handler = h
	}
}

// WithWhatsAppLogger sets the structured logger.
func WithWhatsAppLogger(logger *slog.Logger) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		a.logger = logger
	}
}

// WithWhatsAppHTTPClient sets a custom HTTP client for outbound API requests and media downloading.
func WithWhatsAppHTTPClient(client *http.Client) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		a.client = client
	}
}

// WithWhatsAppTranscriber wires an audio transcriber (e.g. Whisper) for voice notes.
func WithWhatsAppTranscriber(transcriber core.Transcriber) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		a.transcriber = transcriber
	}
}

// WithWhatsAppMaxAudioSize sets the maximum allowed audio download size in bytes.
func WithWhatsAppMaxAudioSize(size int64) WhatsAppOption {
	return func(a *WhatsAppAdapter) {
		if size > 0 {
			a.maxAudioSize = size
		}
	}
}

// WhatsAppAdapter implements the Adapter port for Meta Cloud API WhatsApp webhooks and messaging.
type WhatsAppAdapter struct {
	cfg          WhatsAppConfig
	baseURL      string
	handler      Handler
	logger       *slog.Logger
	client       *http.Client
	transcriber  core.Transcriber
	maxAudioSize int64
	server       *http.Server

	mu      sync.Mutex
	running bool
	closed  bool
}

// NewWhatsAppAdapter constructs a new WhatsAppAdapter.
func NewWhatsAppAdapter(cfg WhatsAppConfig, opts ...WhatsAppOption) *WhatsAppAdapter {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":3003"
	}
	a := &WhatsAppAdapter{
		cfg:          cfg,
		baseURL:      "https://graph.facebook.com/v21.0",
		client:       &http.Client{Timeout: 30 * time.Second},
		logger:       slog.Default(),
		maxAudioSize: DefaultMaxAudioSize,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name implements Adapter.
func (w *WhatsAppAdapter) Name() string {
	return "whatsapp"
}

// Start begins listening for incoming Meta WhatsApp webhook requests.
func (w *WhatsAppAdapter) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrAdapterClosed
	}
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/whatsapp/webhook", w.ServeHTTP)
	mux.HandleFunc("/webhook", w.ServeHTTP)
	mux.HandleFunc("/", w.ServeHTTP)

	w.server = &http.Server{
		Addr:              w.cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", w.cfg.ListenAddr)
	if err != nil {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		return fmt.Errorf("whatsapp adapter: listen on %s: %w", w.cfg.ListenAddr, err)
	}

	w.logger.Info("whatsapp adapter: webhook server listening", "addr", ln.Addr().String())

	go func() {
		if sErr := w.server.Serve(ln); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			w.logger.Error("whatsapp adapter: server error", "error", sErr)
		}
	}()

	return nil
}

// Stop gracefully shuts down the webhook HTTP server.
func (w *WhatsAppAdapter) Stop() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.running = false
	srv := w.server
	w.mu.Unlock()

	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("whatsapp adapter: shutdown: %w", err)
	}
	return nil
}

// Send delivers an outbound message to a recipient phone number via Meta Graph API.
// Messages exceeding WhatsAppMaxMessageLength (4096 runes) are automatically chunked.
func (w *WhatsAppAdapter) Send(ctx context.Context, target string, msg string) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrAdapterClosed
	}
	w.mu.Unlock()

	if target == "" {
		return errors.New("whatsapp adapter: target recipient is empty")
	}

	chunks := SplitMessage(msg, WhatsAppMaxMessageLength)
	for _, chunk := range chunks {
		if err := w.sendSingle(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

type metaSendMessageRequest struct {
	MessagingProduct string           `json:"messaging_product"`
	RecipientType    string           `json:"recipient_type"`
	To               string           `json:"to"`
	Type             string           `json:"type"`
	Text             metaTextPayload  `json:"text"`
}

type metaTextPayload struct {
	Body string `json:"body"`
}

func (w *WhatsAppAdapter) sendSingle(ctx context.Context, target, text string) error {
	apiURL := fmt.Sprintf("%s/%s/messages", w.baseURL, w.cfg.PhoneNumberID)

	reqPayload := metaSendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               target,
		Type:             "text",
		Text:             metaTextPayload{Body: text},
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return fmt.Errorf("whatsapp adapter: marshal message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("whatsapp adapter: create send request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+w.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp adapter: execute send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("whatsapp adapter: send failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ServeHTTP handles inbound Meta Webhook requests (GET for verification, POST for message events).
func (w *WhatsAppAdapter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.handleVerification(rw, r)
	case http.MethodPost:
		w.handleWebhookEvent(rw, r)
	default:
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerification handles Meta's webhook verification GET request.
func (w *WhatsAppAdapter) handleVerification(rw http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" || token == "" || challenge == "" {
		http.Error(rw, "forbidden", http.StatusForbidden)
		return
	}

	if w.cfg.VerifyToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(w.cfg.VerifyToken)) != 1 {
		w.logger.Warn("whatsapp adapter: webhook verification failed (token mismatch)")
		http.Error(rw, "forbidden", http.StatusForbidden)
		return
	}

	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(challenge))
}

// verifyHMAC checks X-Hub-Signature-256 header using constant-time comparison.
func (w *WhatsAppAdapter) verifyHMAC(headerSignature string, body []byte) bool {
	if w.cfg.AppSecret == "" || headerSignature == "" {
		return false
	}
	if !strings.HasPrefix(headerSignature, "sha256=") {
		return false
	}
	expectedSig := headerSignature[7:]

	mac := hmac.New(sha256.New, []byte(w.cfg.AppSecret))
	mac.Write(body)
	computedSig := hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(expectedSig), []byte(computedSig)) == 1
}

// Meta Webhook JSON structures
type metaWebhookPayload struct {
	Object string             `json:"object"`
	Entry  []metaWebhookEntry `json:"entry"`
}

type metaWebhookEntry struct {
	ID      string              `json:"id"`
	Changes []metaWebhookChange `json:"changes"`
}

type metaWebhookChange struct {
	Value metaWebhookValue `json:"value"`
	Field string           `json:"field"`
}

type metaWebhookValue struct {
	MessagingProduct string                `json:"messaging_product"`
	Metadata         metaWebhookMetadata   `json:"metadata"`
	Messages         []metaInboundMessage  `json:"messages"`
}

type metaWebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type metaInboundMessage struct {
	From      string             `json:"from"`
	ID        string             `json:"id"`
	Timestamp string             `json:"timestamp"`
	Type      string             `json:"type"`
	Text      *metaInboundText   `json:"text,omitempty"`
	Audio     *metaInboundMedia  `json:"audio,omitempty"`
	Voice     *metaInboundMedia  `json:"voice,omitempty"`
}

type metaInboundText struct {
	Body string `json:"body"`
}

type metaInboundMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
}

func (w *WhatsAppAdapter) handleWebhookEvent(rw http.ResponseWriter, r *http.Request) {
	// Bounded reading to prevent memory exhaustion (DoS mitigation)
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(rw, "cannot read body", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !w.verifyHMAC(sig, body) {
		w.logger.Warn("whatsapp adapter: invalid X-Hub-Signature-256 signature")
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload metaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.logger.Warn("whatsapp adapter: failed to unmarshal payload", "error", err)
		http.Error(rw, "bad request", http.StatusBadRequest)
		return
	}

	// Meta requires returning HTTP 200 OK immediately for acknowledged events
	rw.WriteHeader(http.StatusOK)

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				w.processInboundMessage(r.Context(), msg)
			}
		}
	}
}

func (w *WhatsAppAdapter) processInboundMessage(ctx context.Context, msg metaInboundMessage) {
	sender := msg.From
	if !IsAllowed(w.cfg.AllowedUsers, sender) {
		w.logger.Warn("whatsapp adapter: dropping message from unauthorized sender", "sender", sender)
		return
	}

	var ts time.Time
	if msg.Timestamp != "" {
		if sec, err := strconv.ParseInt(msg.Timestamp, 10, 64); err == nil {
			ts = time.Unix(sec, 0).UTC()
		}
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	switch msg.Type {
	case "text":
		if msg.Text == nil || msg.Text.Body == "" {
			return
		}
		event := MessageEvent{
			Adapter:   "whatsapp",
			UserID:    sender,
			ChatID:    sender,
			Content:   msg.Text.Body,
			Timestamp: ts,
		}
		w.dispatchEvent(ctx, event)

	case "audio", "voice":
		media := msg.Audio
		if media == nil {
			media = msg.Voice
		}
		if media == nil || media.ID == "" {
			return
		}

		audioData, mimeType, err := w.fetchMedia(ctx, media.ID)
		if err != nil {
			w.logger.Error("whatsapp adapter: downloading media failed", "media_id", media.ID, "error", err)
			return
		}

		content := ""
		if w.transcriber != nil {
			transcription, tErr := w.transcriber.Transcribe(ctx, audioData, mimeType)
			if tErr != nil {
				w.logger.Warn("whatsapp adapter: audio transcription failed", "media_id", media.ID, "error", tErr)
				content = "[Audio message - transcription failed]"
			} else {
				content = transcription
			}
		} else {
			content = "[Audio message received]"
		}

		event := MessageEvent{
			Adapter: sender,
			UserID:  sender,
			ChatID:  sender,
			Content: content,
			Attachments: []core.Attachment{
				{
					Type:     "audio",
					Data:     audioData,
					MimeType: mimeType,
				},
			},
			Timestamp: ts,
		}
		event.Adapter = "whatsapp"
		w.dispatchEvent(ctx, event)
	default:
		w.logger.Debug("whatsapp adapter: ignoring unhandled message type", "type", msg.Type)
	}
}

type metaMediaResponse struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
	ID       string `json:"id"`
}

func (w *WhatsAppAdapter) fetchMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	metaURL := fmt.Sprintf("%s/%s", w.baseURL, mediaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating media lookup request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+w.cfg.APIToken)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing media lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("media lookup failed with HTTP status %d", resp.StatusCode)
	}

	var mediaResp metaMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&mediaResp); err != nil {
		return nil, "", fmt.Errorf("decoding media metadata: %w", err)
	}

	if mediaResp.URL == "" {
		return nil, "", errors.New("empty media download URL in Meta response")
	}

	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaResp.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating download request: %w", err)
	}
	downloadReq.Header.Set("Authorization", "Bearer "+w.cfg.APIToken)

	dlResp, err := w.client.Do(downloadReq)
	if err != nil {
		return nil, "", fmt.Errorf("executing media download: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode < 200 || dlResp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("media download failed with HTTP status %d", dlResp.StatusCode)
	}

	maxBytes := w.maxAudioSize
	if maxBytes <= 0 {
		maxBytes = DefaultMaxAudioSize
	}

	limited := io.LimitReader(dlResp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("reading media bytes: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("%w: received %d bytes exceeding limit %d", ErrMediaTooLarge, len(data), maxBytes)
	}
	if len(data) == 0 {
		return nil, "", ErrEmptyMedia
	}

	mime := mediaResp.MimeType
	if mime == "" {
		mime = dlResp.Header.Get("Content-Type")
	}
	if sniffed := SniffContentType(data); sniffed != "" {
		mime = sniffed
	}

	return data, mime, nil
}

func (w *WhatsAppAdapter) dispatchEvent(ctx context.Context, event MessageEvent) {
	if w.handler == nil {
		return
	}
	if err := w.handler(ctx, event); err != nil {
		w.logger.Error("whatsapp adapter: handler error", "error", err, "sender", event.UserID)
	}
}
