// Package webhook implements the HTTP webhook ingestion server with HMAC-SHA256
// signature verification, sandbox brain dispatch, and optional gateway target delivery.
package webhook

import (
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
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
)

// Target defines an outbound chat destination for webhook results.
type Target struct {
	Adapter   string
	Recipient string
}

// Config specifies webhook server settings.
type Config struct {
	Host             string
	Port             int
	Path             string
	Secret           string
	DefaultSessionID string
	Target           *Target
}

// BrainRunner executes a brain turn.
type BrainRunner interface {
	SetActiveConversation(id string)
	Step(ctx context.Context, input string) error
}

// TargetSender sends outbound messages to chat gateway adapters.
type TargetSender interface {
	Send(ctx context.Context, adapter, target, msg string) error
}

// Option configures a webhook Server.
type Option func(*Server)

// WithBrain sets the Brain execution engine.
func WithBrain(brain BrainRunner) Option {
	return func(s *Server) { s.brain = brain }
}

// WithRepo sets the conversation repository.
func WithRepo(repo core.Repository) Option {
	return func(s *Server) { s.repo = repo }
}

// WithSender sets the gateway message sender for targets.
func WithSender(sender TargetSender) Option {
	return func(s *Server) { s.sender = sender }
}

// WithApprover sets the policy approver for brain turns.
func WithApprover(approver core.Approver) Option {
	return func(s *Server) { s.approver = approver }
}

// WithLogger sets the logger instance.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Server) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// Server is an HTTP server handling incoming webhook requests.
type Server struct {
	cfg          Config
	brain        BrainRunner
	repo         core.Repository
	sender       TargetSender
	approver     core.Approver
	logger       *slog.Logger
	sessions     map[string]string
	sessionLocks map[string]*sync.Mutex
	mu           sync.RWMutex
	httpServer   *http.Server
}

// NewServer constructs a new webhook HTTP Server.
func NewServer(cfg Config, opts ...Option) *Server {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Path == "" {
		cfg.Path = "/webhook"
	}
	if cfg.DefaultSessionID == "" {
		cfg.DefaultSessionID = "webhook-events"
	}

	s := &Server{
		cfg:          cfg,
		logger:       slog.Default(),
		sessions:     make(map[string]string),
		sessionLocks: make(map[string]*sync.Mutex),
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.approver == nil {
		s.approver = gateway.NewAutoDenyApprover(s.logger)
	}

	return s
}

// VerifySignature validates that headerSig matches the HMAC-SHA256 of body using secret.
// Constant-time comparison is enforced to prevent timing attacks.
func VerifySignature(secret string, body []byte, headerSig string) bool {
	if secret == "" {
		return true
	}
	if headerSig == "" {
		return false
	}

	sigHex := strings.TrimPrefix(headerSig, "sha256=")
	sigHex = strings.TrimSpace(sigHex)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if len(sigHex) != len(expectedMAC) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(expectedMAC), []byte(sigHex)) == 1
}

// ServeHTTP handles incoming HTTP requests for webhook ingestion.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != s.cfg.Path {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit body size to 1MB to prevent memory exhaustion
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		s.logger.Warn("webhook: failed reading request body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if s.cfg.Secret != "" {
		sigHeader := r.Header.Get("X-Hub-Signature-256")
		if sigHeader == "" {
			sigHeader = r.Header.Get("X-Signature")
		}

		if !VerifySignature(s.cfg.Secret, body, sigHeader) {
			s.logger.Warn("webhook: HMAC signature mismatch or missing signature header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	s.processEvent(r.Context(), body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) getSessionLock(sessionKey string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.sessionLocks[sessionKey]
	if !ok {
		l = &sync.Mutex{}
		s.sessionLocks[sessionKey] = l
	}
	return l
}

func (s *Server) resolveConversation(ctx context.Context, sessionKey string) (string, error) {
	s.mu.RLock()
	convID, ok := s.sessions[sessionKey]
	s.mu.RUnlock()

	if ok && convID != "" {
		return convID, nil
	}

	if s.repo == nil {
		return "", fmt.Errorf("no repository wired")
	}

	conv, err := s.repo.CreateConversation(ctx, sessionKey)
	if err != nil {
		return "", fmt.Errorf("creating session conversation: %w", err)
	}

	s.mu.Lock()
	s.sessions[sessionKey] = conv.ID
	s.mu.Unlock()

	return conv.ID, nil
}

func (s *Server) processEvent(ctx context.Context, body []byte) {
	sessionKey := s.cfg.DefaultSessionID

	// Try extracting event/type property if payload is JSON
	var payloadMap map[string]any
	if err := json.Unmarshal(body, &payloadMap); err == nil {
		if ev, ok := payloadMap["event"].(string); ok && ev != "" {
			sessionKey = fmt.Sprintf("webhook:%s", ev)
		} else if typ, ok := payloadMap["type"].(string); ok && typ != "" {
			sessionKey = fmt.Sprintf("webhook:%s", typ)
		}
	}

	sLock := s.getSessionLock(sessionKey)
	sLock.Lock()
	defer sLock.Unlock()

	convID, err := s.resolveConversation(ctx, sessionKey)
	if err != nil {
		s.logger.Error("webhook: failed resolving session conversation", "session", sessionKey, "error", err)
		return
	}

	if s.brain == nil {
		s.logger.Warn("webhook: no brain wired, skipping execution")
		return
	}

	s.brain.SetActiveConversation(convID)
	prompt := fmt.Sprintf("Webhook event received: %s", string(body))

	if err := s.brain.Step(ctx, prompt); err != nil {
		s.logger.Error("webhook: brain step execution failed", "session", sessionKey, "error", err)
		return
	}

	var replyText string
	if s.repo != nil {
		msgs, err := s.repo.Messages(ctx, convID, 1)
		if err == nil && len(msgs) > 0 && msgs[0].Role == core.RoleAssistant {
			replyText = msgs[0].Content
		}
	}

	if s.cfg.Target != nil && s.cfg.Target.Adapter != "" && s.cfg.Target.Recipient != "" {
		if s.sender != nil {
			if err := s.sender.Send(ctx, s.cfg.Target.Adapter, s.cfg.Target.Recipient, replyText); err != nil {
				s.logger.Error("webhook: target send failed",
					"adapter", s.cfg.Target.Adapter,
					"recipient", s.cfg.Target.Recipient,
					"error", err,
				)
				return
			}
			s.logger.Info("webhook: notification delivered to target",
				"adapter", s.cfg.Target.Adapter,
				"recipient", s.cfg.Target.Recipient,
			)
		} else {
			s.logger.Warn("webhook: target configured but no sender wired")
		}
	} else {
		s.logger.Info("webhook: event processed", "session", sessionKey, "output", replyText)
	}
}

// Start launches the HTTP server and blocks until ctx is canceled, executing graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("webhook server listening on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.httpServer = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.mu.Unlock()

	s.logger.Info("webhook server: listening", "addr", listener.Addr().String(), "path", s.cfg.Path)

	errCh := make(chan error, 1)
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("webhook server: shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		s.mu.Lock()
		srv := s.httpServer
		s.mu.Unlock()

		if srv != nil {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				s.logger.Warn("webhook server: graceful shutdown error", "error", err)
			}
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Stop initiates graceful shutdown of the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}
