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
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// SlackMaxMessageLength is Slack's per-message character limit (4000 runes).
const SlackMaxMessageLength = 4000

// SlackConfig carries Slack adapter configuration.
type SlackConfig = config.SlackConfig

// SlackOption configures a SlackAdapter.
type SlackOption func(*SlackAdapter)

// WithSlackBaseURL sets a custom Slack API base URL (useful in tests).
func WithSlackBaseURL(url string) SlackOption {
	return func(a *SlackAdapter) {
		a.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithSlackHandler sets the message event handler.
func WithSlackHandler(h Handler) SlackOption {
	return func(a *SlackAdapter) {
		a.handler = h
	}
}

// WithSlackLogger sets the structured logger.
func WithSlackLogger(logger *slog.Logger) SlackOption {
	return func(a *SlackAdapter) {
		a.logger = logger
	}
}

// WithSlackHTTPClient sets a custom HTTP client for outbound API requests.
func WithSlackHTTPClient(client *http.Client) SlackOption {
	return func(a *SlackAdapter) {
		a.client = client
	}
}

// SlackAdapter implements the Adapter port for Slack Events API and Web API.
type SlackAdapter struct {
	cfg     SlackConfig
	baseURL string
	handler Handler
	logger  *slog.Logger
	client  *http.Client
	server  *http.Server

	mu      sync.Mutex
	running bool
	closed  bool
}

// NewSlackAdapter constructs a new SlackAdapter.
func NewSlackAdapter(cfg SlackConfig, opts ...SlackOption) *SlackAdapter {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":3002"
	}
	a := &SlackAdapter{
		cfg:     cfg,
		baseURL: "https://slack.com/api",
		client:  &http.Client{Timeout: 30 * time.Second},
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name implements Adapter.
func (s *SlackAdapter) Name() string {
	return "slack"
}

// Start begins listening for incoming Slack Events API webhook requests.
func (s *SlackAdapter) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("slack adapter already started")
	}
	if s.closed {
		s.mu.Unlock()
		return ErrAdapterClosed
	}

	listener, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("starting slack listener on %s: %w", s.cfg.ListenAddr, err)
	}

	s.server = &http.Server{
		Handler:      s,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("slack server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server and stops accepting requests.
func (s *SlackAdapter) Stop() error {
	s.mu.Lock()
	if !s.running && s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.running = false
	srv := s.server
	s.mu.Unlock()

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down slack server: %w", err)
		}
	}

	return nil
}

// Send transmits an outbound message to the specified Slack channel or DM target.
func (s *SlackAdapter) Send(ctx context.Context, target string, msg string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrAdapterClosed
	}
	s.mu.Unlock()

	chunks := SplitMessage(msg, SlackMaxMessageLength)
	for _, chunk := range chunks {
		if err := s.postMessageChunk(ctx, target, chunk); err != nil {
			return err
		}
	}

	return nil
}

func (s *SlackAdapter) postMessageChunk(ctx context.Context, channel, text string) error {
	payload := map[string]string{
		"channel": channel,
		"text":    text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling slack postMessage payload: %w", err)
	}

	endpoint := s.baseURL + "/chat.postMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating slack postMessage request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if s.cfg.BotToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.BotToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing slack postMessage: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return fmt.Errorf("reading slack postMessage response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack postMessage failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("parsing slack postMessage response: %w", err)
	}

	if !apiResp.OK {
		return fmt.Errorf("slack api error: %s", apiResp.Error)
	}

	return nil
}

// ServeHTTP handles incoming Slack Events API requests.
func (s *SlackAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Verify timestamp freshness (< 300s)
	tsHeader := r.Header.Get("X-Slack-Request-Timestamp")
	tsInt, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		http.Error(w, "invalid timestamp", http.StatusBadRequest)
		return
	}

	now := time.Now().Unix()
	if math.Abs(float64(now-tsInt)) > 300 {
		http.Error(w, "request timestamp expired", http.StatusBadRequest)
		return
	}

	// 2. Read raw request body (limit to 10MB)
	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, "failed reading body", http.StatusBadRequest)
		return
	}

	// 3. Constant-time HMAC-SHA256 signature verification
	sigHeader := r.Header.Get("X-Slack-Signature")
	if sigHeader == "" {
		http.Error(w, "missing signature", http.StatusUnauthorized)
		return
	}

	if !s.verifySignature(sigHeader, tsInt, bodyBytes) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// 4. Parse payload
	var rawPayload struct {
		Type      string          `json:"type"`
		Challenge string          `json:"challenge"`
		Event     json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(bodyBytes, &rawPayload); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	// Handle URL verification handshake
	if rawPayload.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": rawPayload.Challenge})
		return
	}

	// Handle Event Callback
	if rawPayload.Type == "event_callback" {
		var event struct {
			Type     string `json:"type"`
			Subtype  string `json:"subtype"`
			BotID    string `json:"bot_id"`
			User     string `json:"user"`
			Channel  string `json:"channel"`
			Text     string `json:"text"`
			ThreadTS string `json:"thread_ts"`
			TS       string `json:"ts"`
		}
		if err := json.Unmarshal(rawPayload.Event, &event); err != nil {
			s.logger.Warn("slack: failed parsing event", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Filter bot messages to prevent loops
		if event.BotID != "" || event.Subtype == "bot_message" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Filter allowlist
		if !IsAllowed(s.cfg.AllowedUsers, event.User) {
			s.logger.Warn("slack: drop unauthorized message", "user", event.User, "channel", event.Channel)
			w.WriteHeader(http.StatusOK)
			return
		}

		if s.handler != nil {
			msgEvent := MessageEvent{
				Adapter:   "slack",
				UserID:    event.User,
				ChatID:    event.Channel,
				Content:   event.Text,
				Timestamp: time.Now(),
			}
			if err := s.handler(r.Context(), msgEvent); err != nil {
				s.logger.Error("slack: error handling event", "error", err)
			}
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *SlackAdapter) verifySignature(sigHeader string, timestamp int64, body []byte) bool {
	sigBase := fmt.Sprintf("v0:%d:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(s.cfg.SigningSecret))
	mac.Write([]byte(sigBase))
	expectedSig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(expectedSig), []byte(sigHeader)) == 1
}
