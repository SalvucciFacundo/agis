package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/scan"
)

// Manager owns the active session id and exposes the 7 slash operations.
// It is surface-agnostic: TUI, gateway and cron share the same manager.
type Manager struct {
	repo    core.Repository
	closer  core.SessionCloser
	logger  *slog.Logger
	activeID string
}

// New returns a Manager. closer may be nil, which disables Compress.
func New(repo core.Repository, closer core.SessionCloser, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{repo: repo, closer: closer, logger: logger}
}

// ActiveID returns the current active conversation id, or empty if none.
func (m *Manager) ActiveID() string { return m.activeID }

// SetActive switches the active conversation.
func (m *Manager) SetActive(id string) { m.activeID = id }

// NewSession creates a fresh conversation and makes it active. It mirrors
// `ensureConversation` but always creates.
func (m *Manager) NewSession(ctx context.Context) (*core.Conversation, error) {
	conv, err := m.repo.CreateConversation(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	m.activeID = conv.ID
	return conv, nil
}

// Save ensures the active session exists; messages already persist
// incrementally, so Save is a no-op beyond validation.
func (m *Manager) Save(ctx context.Context) error {
	if m.activeID == "" {
		conv, err := m.repo.CreateConversation(ctx, "")
		if err != nil {
			return err
		}
		m.activeID = conv.ID
		return nil
	}
	if _, err := m.repo.GetConversation(ctx, m.activeID); err != nil {
		return err
	}
	return nil
}

// List returns recent conversations ordered updated_at DESC, id DESC.
func (m *Manager) List(ctx context.Context, limit int) ([]core.Conversation, error) {
	return m.repo.ListConversations(ctx, limit, 0)
}

// Restore validates and switches to an existing conversation.
func (m *Manager) Restore(ctx context.Context, id string) error {
	if _, err := m.repo.GetConversation(ctx, id); err != nil {
		return err
	}
	m.activeID = id
	return nil
}

// Rename updates a conversation title after scanning for injection.
func (m *Manager) Rename(ctx context.Context, id, title string) error {
	clean, dropped := scan.Lines(title)
	if dropped > 0 {
		m.logger.Warn("session: dropped injected lines from title", "dropped", dropped)
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return fmt.Errorf("title must not be empty")
	}
	return m.repo.RenameConversation(ctx, id, clean)
}

// Compress runs the summarizer early on the active session's tail.
func (m *Manager) Compress(ctx context.Context) error {
	if m.closer == nil {
		return fmt.Errorf("no summarizer wired")
	}
	if m.activeID == "" {
		return fmt.Errorf("no active session")
	}
	msgs, err := m.repo.Messages(ctx, m.activeID, 200)
	if err != nil {
		return err
	}
	return m.closer.Close(ctx, m.activeID, msgs)
}

// Snapshot captures a point-in-time copy of the active conversation.
func (m *Manager) Snapshot(ctx context.Context) (*core.Snapshot, error) {
	if m.activeID == "" {
		return nil, fmt.Errorf("no active session")
	}
	return m.repo.CreateSnapshot(ctx, m.activeID)
}
