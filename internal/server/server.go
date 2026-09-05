package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// Options configures the API server instance.
type Options struct {
	Host         string
	Port         int
	APIKey       string
	CORSOrigins  []string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Brain        *core.Brain
	Logger       *slog.Logger
	Profile      string
	Version      string
	Provider     string
	Model        string
}

// Server is the AGIS OpenAI-compatible REST API server.
type Server struct {
	opts       Options
	httpServer *http.Server
	handler    http.Handler
	listener   net.Listener
	mu         sync.Mutex
}

// New constructs an initialized Server with applied defaults and route bindings.
func New(opts Options) *Server {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = 30 * time.Second
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 120 * time.Second
	}
	if opts.Profile == "" {
		opts.Profile = "default"
	}
	if opts.Version == "" {
		opts.Version = "0.1.0"
	}

	s := &Server{
		opts: opts,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/health", s.handleHealth)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)

	s.handler = CORSMiddleware(opts.CORSOrigins)(
		AuthMiddleware(opts.APIKey)(mux),
	)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", opts.Host, opts.Port),
		Handler:      s.handler,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}

	return s
}

// Handler returns the fully-wrapped HTTP handler with all middleware applied.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Start binds the listener and begins serving incoming HTTP requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("binding listener on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	return s.httpServer.Serve(ln)
}

// Shutdown gracefully drains active connections up to the deadline in ctx.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resp := HealthResponse{
		Status:         "ok",
		Version:        s.opts.Version,
		Profile:        s.opts.Profile,
		ActiveProvider: s.opts.Provider,
		ActiveModel:    s.opts.Model,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	modelID := s.opts.Model
	if modelID == "" {
		modelID = "llama3.2"
	}
	provider := s.opts.Provider
	if provider == "" {
		provider = "ollama"
	}

	resp := ModelListResponse{
		Object: "list",
		Data: []ModelItem{
			{
				ID:      modelID,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: provider,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
