package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/session"
)

func newSessionRepo(t *testing.T) *memory.Repository {
	t.Helper()
	repo, err := memory.NewRepository(context.Background(), t.TempDir()+"/agis.db")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func newSessionTestModel(t *testing.T, repo *memory.Repository) (*Model, *session.Manager) {
	t.Helper()
	mgr := session.New(repo, &fakeSessionCloser{}, nil)
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSink(func(string) {}))
	m := New(brain, repo, stream, WithSessionManager(mgr))
	if conv, err := repo.LatestConversation(context.Background()); err == nil {
		mgr.SetActive(conv.ID)
		brain.SetActiveConversation(conv.ID)
	}
	return m, mgr
}

type fakeSessionCloser struct{ calls int }

func (f *fakeSessionCloser) Close(ctx context.Context, convID string, msgs []core.Message) error {
	f.calls++
	return nil
}

func sendSlash(t *testing.T, m *Model, line string) *Model {
	t.Helper()
	m.input.SetValue(line)
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return model.(*Model)
}

func TestSlash_NewCreatesAndSwitches(t *testing.T) {
	repo := newSessionRepo(t)
	m, mgr := newSessionTestModel(t, repo)

	m = sendSlash(t, m, "/new")
	if mgr.ActiveID() == "" {
		t.Fatal("activeID empty after /new")
	}
	if !strings.Contains(m.history.String(), "new session") {
		t.Errorf("feedback missing: %q", m.history.String())
	}
	convs, _ := repo.ListConversations(context.Background(), 10, 0)
	if len(convs) != 1 {
		t.Fatalf("conversations = %d, want 1", len(convs))
	}
}

func TestSlash_ListShowsTitles(t *testing.T) {
	repo := newSessionRepo(t)
	m, mgr := newSessionTestModel(t, repo)
	mgr.NewSession(context.Background())
	mgr.Rename(context.Background(), mgr.ActiveID(), "Alpha")
	mgr.NewSession(context.Background())
	mgr.Rename(context.Background(), mgr.ActiveID(), "Beta")

	m = sendSlash(t, m, "/list")
	hist := m.history.String()
	if !strings.Contains(hist, "Alpha") || !strings.Contains(hist, "Beta") {
		t.Errorf("list output missing titles: %q", hist)
	}
}

func TestSlash_RenameScansInjection(t *testing.T) {
	repo := newSessionRepo(t)
	m, mgr := newSessionTestModel(t, repo)
	conv, _ := mgr.NewSession(context.Background())
	m.brain.SetActiveConversation(conv.ID)

	m = sendSlash(t, m, "/rename My Research")
	got, _ := repo.GetConversation(context.Background(), conv.ID)
	if got.Title != "My Research" {
		t.Errorf("title = %q, want My Research", got.Title)
	}
	// Injection scrubbing is covered by manager unit test with multiline titles;
	// TUI's single-line input cannot produce the multiline injection case.
}

func TestSlash_GatedWhileStreaming(t *testing.T) {
	repo := newSessionRepo(t)
	m, _ := newSessionTestModel(t, repo)
	m.streaming = true

	m = sendSlash(t, m, "/new")
	if strings.Contains(m.history.String(), "new session") {
		t.Error("gated command executed while streaming")
	}
}

func TestSlash_CompressCallsCloser(t *testing.T) {
	repo := newSessionRepo(t)
	closer := &fakeSessionCloser{}
	mgr := session.New(repo, closer, nil)
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSink(func(string) {}))
	m := New(brain, repo, stream, WithSessionManager(mgr))
	mgr.NewSession(context.Background())
	brain.SetActiveConversation(mgr.ActiveID())
	_ = repo.AppendMessage(context.Background(), mgr.ActiveID(), core.Message{Role: core.RoleUser, Content: "hi"})

	m = sendSlash(t, m, "/compress")
	if closer.calls != 1 {
		t.Errorf("compress calls = %d, want 1", closer.calls)
	}
	if !strings.Contains(m.history.String(), "compressed") {
		t.Errorf("feedback missing: %q", m.history.String())
	}
}

func TestSlash_RestoreLoadsHistory(t *testing.T) {
	repo := newSessionRepo(t)
	m, mgr := newSessionTestModel(t, repo)
	c1, _ := mgr.NewSession(context.Background())
	_ = repo.AppendMessage(context.Background(), c1.ID, core.Message{Role: core.RoleUser, Content: "hello from c1"})
	_, _ = mgr.NewSession(context.Background())

	m = sendSlash(t, m, "/restore "+c1.ID)
	if mgr.ActiveID() != c1.ID {
		t.Errorf("activeID = %q, want %q", mgr.ActiveID(), c1.ID)
	}
	if !strings.Contains(m.history.String(), "hello from c1") {
		t.Errorf("history missing restored messages: %q", m.history.String())
	}
}
