package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/scan"
)

// ExportFormat represents supported export serialization formats.
type ExportFormat string

const (
	ExportFormatJSON     ExportFormat = "json"
	ExportFormatMarkdown ExportFormat = "markdown"
	ExportFormatTXT      ExportFormat = "txt"
)

// SessionExport represents the structured export payload for JSON formatting.
type SessionExport struct {
	Conversation *core.Conversation `json:"conversation"`
	Messages     []core.Message     `json:"messages"`
}

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

// SnapshotSession captures a point-in-time copy of the specified conversation.
func (m *Manager) SnapshotSession(ctx context.Context, id string) (*core.Snapshot, error) {
	return m.repo.CreateSnapshot(ctx, id)
}

// Snapshot captures a point-in-time copy of the active conversation.
func (m *Manager) Snapshot(ctx context.Context) (*core.Snapshot, error) {
	if m.activeID == "" {
		return nil, fmt.Errorf("no active session")
	}
	return m.SnapshotSession(ctx, m.activeID)
}

// Show retrieves conversation metadata and the complete message history without
// altering the activeID.
func (m *Manager) Show(ctx context.Context, id string) (*core.Conversation, []core.Message, error) {
	conv, err := m.repo.GetConversation(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := m.repo.Messages(ctx, id, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("loading messages for %s: %w", id, err)
	}
	return conv, msgs, nil
}

// Delete permanently removes a conversation and cascades to all linked messages,
// snapshots, and attachments. If the deleted session is currently active, activeID is reset.
func (m *Manager) Delete(ctx context.Context, id string) error {
	if err := m.repo.DeleteConversation(ctx, id); err != nil {
		return err
	}
	if m.activeID == id {
		m.activeID = ""
	}
	return nil
}

// Export serializes the conversation and its message history into the requested format.
func (m *Manager) Export(ctx context.Context, id string, format ExportFormat) ([]byte, error) {
	conv, msgs, err := m.Show(ctx, id)
	if err != nil {
		return nil, err
	}

	normFormat := strings.ToLower(strings.TrimSpace(string(format)))
	switch normFormat {
	case "json":
		payload := SessionExport{
			Conversation: conv,
			Messages:     msgs,
		}
		return json.MarshalIndent(payload, "", "  ")

	case "markdown", "md":
		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", conv.Title)
		fmt.Fprintf(&b, "- **ID:** `%s`\n", conv.ID)
		fmt.Fprintf(&b, "- **Created:** %s\n", conv.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
		fmt.Fprintf(&b, "- **Updated:** %s\n", conv.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
		if conv.Summary != "" {
			fmt.Fprintf(&b, "- **Summary:** %s\n", conv.Summary)
		}
		b.WriteString("\n---\n\n")

		for _, msg := range msgs {
			roleTitle := string(msg.Role)
			if len(roleTitle) > 0 {
				roleTitle = strings.ToUpper(roleTitle[:1]) + roleTitle[1:]
			}
			fmt.Fprintf(&b, "### %s (%s)\n\n", roleTitle, msg.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
			b.WriteString(msg.Content)
			b.WriteString("\n\n")

			if len(msg.Attachments) > 0 {
				b.WriteString("**Attachments:**\n")
				for _, att := range msg.Attachments {
					if att.URL != "" {
						fmt.Fprintf(&b, "- [%s](%s) (%s)\n", att.Name, att.URL, att.MimeType)
					} else {
						fmt.Fprintf(&b, "- %s (%s)\n", att.Name, att.MimeType)
					}
				}
				b.WriteString("\n")
			}
		}
		return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil

	case "txt", "plaintext":
		var b strings.Builder
		fmt.Fprintf(&b, "Session: %s (%s)\n", conv.Title, conv.ID)
		fmt.Fprintf(&b, "Created: %s | Updated: %s\n", conv.CreatedAt.Format("2006-01-02 15:04:05 UTC"), conv.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
		if conv.Summary != "" {
			fmt.Fprintf(&b, "Summary: %s\n", conv.Summary)
		}
		b.WriteString(strings.Repeat("-", 60) + "\n\n")

		for _, msg := range msgs {
			fmt.Fprintf(&b, "[%s] [%s]: %s\n", msg.CreatedAt.Format("2006-01-02 15:04:05 UTC"), strings.ToUpper(string(msg.Role)), msg.Content)
			for _, att := range msg.Attachments {
				fmt.Fprintf(&b, "  [attachment: %s (%s)]\n", att.Name, att.MimeType)
			}
			b.WriteString("\n")
		}
		return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil

	default:
		return nil, fmt.Errorf("invalid export format %q: supported formats are json, markdown, txt", format)
	}
}
