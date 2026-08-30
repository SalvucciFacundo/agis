package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/session"
)

// BrainRunner represents the interface required to execute user turns in the Brain.
type BrainRunner interface {
	Step(ctx context.Context, input string) error
	SetActiveConversation(id string)
}

// Multiplexer coordinates chat platform adapters, orchestrates message routing,
// and maps conversations to AGIS Brain turns.
type Multiplexer struct {
	mu             sync.RWMutex
	adapters       map[string]Adapter
	brain          BrainRunner
	repo           core.Repository
	sessionManager *session.Manager
	logger         *slog.Logger

	// sessions maps sessionKey ("gateway:<adapter>:<chatID>") -> conversationID
	sessions    map[string]string
	sessionLocks map[string]*sync.Mutex

	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

// MultiplexerOption configures a Multiplexer.
type MultiplexerOption func(*Multiplexer)

// WithMultiplexerBrain wires the Brain runner into the Multiplexer.
func WithMultiplexerBrain(b BrainRunner) MultiplexerOption {
	return func(m *Multiplexer) {
		m.brain = b
	}
}

// WithMultiplexerRepository wires the core Repository for session persistence.
func WithMultiplexerRepository(r core.Repository) MultiplexerOption {
	return func(m *Multiplexer) {
		m.repo = r
	}
}

// WithMultiplexerSessionManager wires the session manager.
func WithMultiplexerSessionManager(sm *session.Manager) MultiplexerOption {
	return func(m *Multiplexer) {
		m.sessionManager = sm
	}
}

// WithMultiplexerLogger sets the multiplexer logger.
func WithMultiplexerLogger(l *slog.Logger) MultiplexerOption {
	return func(m *Multiplexer) {
		m.logger = l
	}
}

// NewMultiplexer constructs a new Multiplexer instance.
func NewMultiplexer(opts ...MultiplexerOption) *Multiplexer {
	m := &Multiplexer{
		adapters:     make(map[string]Adapter),
		sessions:     make(map[string]string),
		sessionLocks: make(map[string]*sync.Mutex),
		logger:       slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// RegisterAdapter registers an adapter with the multiplexer.
func (m *Multiplexer) RegisterAdapter(a Adapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[a.Name()] = a
}

// Start initializes and starts all registered adapters.
func (m *Multiplexer) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrAdapterClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	adapters := make([]Adapter, 0, len(m.adapters))
	for _, a := range m.adapters {
		adapters = append(adapters, a)
	}
	m.mu.Unlock()

	m.logger.Info("gateway multiplexer: starting adapters", "count", len(adapters))

	var started []Adapter
	for _, a := range adapters {
		if err := a.Start(ctx); err != nil {
			m.logger.Error("gateway multiplexer: adapter start failed", "adapter", a.Name(), "error", err)
			// Roll back already started adapters
			for _, s := range started {
				_ = s.Stop()
			}
			return err
		}
		started = append(started, a)
	}

	return nil
}

// Stop coordinates graceful shutdown across all registered adapters.
func (m *Multiplexer) Stop() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	adapters := make([]Adapter, 0, len(m.adapters))
	for _, a := range m.adapters {
		adapters = append(adapters, a)
	}
	m.mu.Unlock()

	m.logger.Info("gateway multiplexer: stopping adapters", "count", len(adapters))

	var errMu sync.Mutex
	var firstErr error

	var stopWg sync.WaitGroup
	for _, a := range adapters {
		stopWg.Add(1)
		go func(ad Adapter) {
			defer stopWg.Done()
			if err := ad.Stop(); err != nil {
				m.logger.Warn("gateway multiplexer: adapter stop error", "adapter", ad.Name(), "error", err)
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}(a)
	}
	stopWg.Wait()
	m.wg.Wait()

	m.logger.Info("gateway multiplexer: all adapters stopped cleanly")
	return firstErr
}

// Send routes an outbound message to the specified adapter.
func (m *Multiplexer) Send(ctx context.Context, adapterName string, target string, msg string) error {
	m.mu.RLock()
	adapter, ok := m.adapters[adapterName]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrAdapterNotFound, adapterName)
	}

	return adapter.Send(ctx, target, msg)
}

// GetSessionConvID returns the conversation ID bound to a session key, or empty if none.
func (m *Multiplexer) GetSessionConvID(sessionKey string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionKey]
}

// getSessionLock returns a per-session mutex to serialize turns on the same chat.
func (m *Multiplexer) getSessionLock(sessionKey string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.sessionLocks[sessionKey]
	if !ok {
		l = &sync.Mutex{}
		m.sessionLocks[sessionKey] = l
	}
	return l
}

// HandleEvent processes an inbound message event from any adapter.
func (m *Multiplexer) HandleEvent(ctx context.Context, ev MessageEvent) error {
	sessionKey := fmt.Sprintf("gateway:%s:%s", ev.Adapter, ev.ChatID)

	// Serialize turns for the same session to avoid message interleaving
	sLock := m.getSessionLock(sessionKey)
	sLock.Lock()
	defer sLock.Unlock()

	convID, err := m.resolveConversation(ctx, sessionKey)
	if err != nil {
		return fmt.Errorf("resolving session conversation: %w", err)
	}

	if m.brain == nil {
		m.logger.Warn("gateway multiplexer: no brain wired, dropping event", "session_key", sessionKey)
		return nil
	}

	m.brain.SetActiveConversation(convID)

	var stepErr error
	if bw, ok := m.brain.(interface {
		StepWithAttachments(ctx context.Context, input string, attachments []core.Attachment) error
	}); ok && len(ev.Attachments) > 0 {
		stepErr = bw.StepWithAttachments(ctx, ev.Content, ev.Attachments)
	} else {
		stepErr = m.brain.Step(ctx, ev.Content)
	}

	if stepErr != nil {
		m.logger.Error("gateway multiplexer: brain step error", "session_key", sessionKey, "error", stepErr)
		return fmt.Errorf("brain step: %w", stepErr)
	}

	// Fetch assistant reply to send back via the originating adapter
	if m.repo != nil {
		msgs, err := m.repo.Messages(ctx, convID, 1)
		if err == nil && len(msgs) > 0 && msgs[0].Role == core.RoleAssistant {
			replyText := msgs[0].Content
			if err := m.Send(ctx, ev.Adapter, ev.ChatID, replyText); err != nil {
				m.logger.Error("gateway multiplexer: sending reply failed", "adapter", ev.Adapter, "target", ev.ChatID, "error", err)
				return fmt.Errorf("sending reply: %w", err)
			}
		}
	}

	return nil
}

func (m *Multiplexer) resolveConversation(ctx context.Context, sessionKey string) (string, error) {
	m.mu.RLock()
	convID, ok := m.sessions[sessionKey]
	m.mu.RUnlock()

	if ok && convID != "" {
		return convID, nil
	}

	if m.repo == nil {
		return "", fmt.Errorf("no repository wired")
	}

	// Create conversation with title matching sessionKey
	conv, err := m.repo.CreateConversation(ctx, sessionKey)
	if err != nil {
		return "", fmt.Errorf("creating session conversation: %w", err)
	}

	m.mu.Lock()
	m.sessions[sessionKey] = conv.ID
	m.mu.Unlock()

	return conv.ID, nil
}
