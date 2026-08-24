package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fakeProvider is a test double for the core.Provider port. It streams a fixed
// sequence of events, or fails immediately.
type fakeProvider struct {
	events    []core.StreamEvent
	streamErr error
}

var _ core.Provider = (*fakeProvider)(nil)

func (f *fakeProvider) Chat(context.Context, core.ChatRequest) (core.ChatResponse, error) {
	return core.ChatResponse{}, errors.New("Chat not used in TUI tests")
}

func (f *fakeProvider) Stream(ctx context.Context, _ core.ChatRequest) (<-chan core.StreamEvent, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan core.StreamEvent)
	go func() {
		defer close(ch)
		for _, ev := range f.events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (f *fakeProvider) Models() []core.ModelInfo { return nil }

// fakeRepo is an in-memory test double for the core.Repository port.
type fakeRepo struct {
	conv     *core.Conversation
	messages []core.Message
	err      error
}

var _ core.Repository = (*fakeRepo)(nil)

func (r *fakeRepo) CreateConversation(context.Context, string) (*core.Conversation, error) {
	return &core.Conversation{ID: "conv-1"}, nil
}

func (r *fakeRepo) LatestConversation(context.Context) (*core.Conversation, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.conv == nil {
		return nil, core.ErrNotFound
	}
	return r.conv, nil
}

func (r *fakeRepo) AppendMessage(context.Context, string, core.Message) error { return nil }

func (r *fakeRepo) Messages(context.Context, string, int) ([]core.Message, error) {
	return r.messages, nil
}

func (r *fakeRepo) Search(context.Context, string, int) ([]core.SearchResult, error) {
	return nil, nil
}

func (r *fakeRepo) SaveObservations(context.Context, string, []core.Observation) error { return nil }

func (r *fakeRepo) Observations(context.Context, int) ([]core.Observation, error) { return nil, nil }

func (r *fakeRepo) UpdateConversationSummary(context.Context, string, string) error { return nil }

func (r *fakeRepo) UpsertUserModel(context.Context, []core.UserModel) error { return nil }

func (r *fakeRepo) RecordSessionEvent(context.Context, string, string, string) error { return nil }

func (r *fakeRepo) Close() error { return nil }

// newTestModel wires a Model around a fake provider that streams the given
// events, plus a stream channel that the Brain's sink writes into.
func newTestModel(t *testing.T, repo core.Repository, events []core.StreamEvent) *Model {
	t.Helper()
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{events: events}, core.WithSink(func(text string) {
		stream <- text
	}))
	return New(brain, repo, stream)
}

func TestRestoreHistory(t *testing.T) {
	repo := &fakeRepo{
		conv: &core.Conversation{ID: "conv-1"},
		messages: []core.Message{
			{Role: core.RoleUser, Content: "Hi"},
			{Role: core.RoleAssistant, Content: "Hello"},
		},
	}
	m := newTestModel(t, repo, nil)

	msg := m.loadHistory()()
	m.Update(msg)

	got := m.history.String()
	if !strings.Contains(got, "you: Hi") {
		t.Errorf("history = %q, want it to contain the user message", got)
	}
	if !strings.Contains(got, "assistant: Hello") {
		t.Errorf("history = %q, want it to contain the assistant message", got)
	}
}

func TestRestoreHistory_EmptyIsFine(t *testing.T) {
	repo := &fakeRepo{}
	m := newTestModel(t, repo, nil)

	msg := m.loadHistory()()
	m.Update(msg)

	if m.history.Len() != 0 {
		t.Errorf("history = %q, want empty for no latest conversation", m.history.String())
	}
}

func TestEnter_StreamsIntoViewport(t *testing.T) {
	repo := &fakeRepo{}
	m := newTestModel(t, repo, []core.StreamEvent{{Text: "Hel"}, {Text: "lo"}})
	m.input.SetValue("Hi")

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	m = drive(t, m, cmd)

	got := m.history.String()
	if !strings.Contains(got, "you: Hi") {
		t.Errorf("history = %q, want it to contain the user message", got)
	}
	if !strings.Contains(got, "assistant: Hello") {
		t.Errorf("history = %q, want it to contain the streamed assistant reply", got)
	}
	if m.streaming {
		t.Error("streaming = true, want false after the reply completes")
	}
}

func TestEnter_BlankInputIsIgnored(t *testing.T) {
	repo := &fakeRepo{}
	m := newTestModel(t, repo, nil)
	m.input.SetValue("   ")

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	if cmd != nil {
		t.Errorf("cmd = %v, want nil for blank input", cmd)
	}
	if m.streaming {
		t.Error("streaming = true, want false for blank input")
	}
}

func TestEnter_StreamErrorShowsError(t *testing.T) {
	repo := &fakeRepo{}
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{streamErr: errors.New("boom")}, core.WithSink(func(text string) {
		stream <- text
	}))
	m := New(brain, repo, stream)
	m.input.SetValue("Hi")

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	m = drive(t, m, cmd)

	got := m.history.String()
	if !strings.Contains(got, "error:") || !strings.Contains(got, "boom") {
		t.Errorf("history = %q, want an error line mentioning the failure", got)
	}
	if m.streaming {
		t.Error("streaming = true, want false after the error")
	}
}

// drive runs cmd and feeds each resulting message through Update until the
// stream completes. submit returns only the token reader, so this path never
// encounters the spinner's timer command.
func drive(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		model, next := m.Update(msg)
		m = model.(*Model)
		if _, done := msg.(streamDoneMsg); done {
			return m
		}
		cmd = next
	}
	t.Fatal("command chain ended before streamDoneMsg")
	return nil
}

// stubCloser is a SessionCloser double that counts Close invocations.
type stubCloser struct {
	calls int
}

func (s *stubCloser) Close(context.Context, string, []core.Message) error {
	s.calls++
	return nil
}

func TestWithCloseTimeout(t *testing.T) {
	repo := &fakeRepo{}
	m := newTestModel(t, repo, nil)
	if m.closeTimeout != 30*time.Second {
		t.Errorf("default closeTimeout = %v, want 30s", m.closeTimeout)
	}

	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSink(func(string) {}))
	m = New(brain, repo, stream, WithCloseTimeout(5*time.Second))
	if m.closeTimeout != 5*time.Second {
		t.Errorf("closeTimeout = %v, want 5s", m.closeTimeout)
	}

	m = New(brain, repo, stream, WithCloseTimeout(0))
	if m.closeTimeout != 30*time.Second {
		t.Errorf("closeTimeout(0) = %v, want default 30s", m.closeTimeout)
	}
}

// runQuitKey presses quit on an idle model and drives the close sequence to
// the actual tea.QuitMsg, asserting the closer ran exactly once.
func runQuitKey(t *testing.T, key tea.KeyType) {
	t.Helper()
	repo := &fakeRepo{conv: &core.Conversation{ID: "conv-1"}}
	closer := &stubCloser{}
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSessionCloser(closer))
	m := New(brain, repo, stream)

	model, cmd := m.Update(tea.KeyMsg{Type: key})
	m = model.(*Model)

	if !m.closing {
		t.Fatal("closing = false after first quit press")
	}
	if !strings.Contains(m.View(), "closing") {
		t.Errorf("View() = %q, want a closing status line", m.View())
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the close-session command")
	}
	if closer.calls != 0 {
		t.Fatalf("Close called %d times before the command ran", closer.calls)
	}

	msg := cmd()
	if _, ok := msg.(closedMsg); !ok {
		t.Fatalf("close cmd msg = %T, want closedMsg", msg)
	}
	model, cmd = m.Update(msg)
	m = model.(*Model)
	if cmd == nil {
		t.Fatal("cmd = nil after closedMsg, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("final cmd msg = %T, want tea.QuitMsg", msg)
	}
	if closer.calls != 1 {
		t.Errorf("Close called %d times, want 1", closer.calls)
	}
}

func TestQuit_IdleRunsCloseSessionThenQuits(t *testing.T) {
	runQuitKey(t, tea.KeyCtrlC)
	runQuitKey(t, tea.KeyEsc)
}

func TestQuit_IdleTwiceForceQuits(t *testing.T) {
	repo := &fakeRepo{}
	closer := &stubCloser{}
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSessionCloser(closer))
	m := New(brain, repo, stream)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)

	// Second press while the close is still scheduled: force quit.
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("cmd = nil, want an immediate quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("second press did not produce tea.QuitMsg")
	}
	_ = m
}

// driveUntilStreamDone runs the token chain until Step finishes.
func driveUntilStreamDone(t *testing.T, m *Model, cmd tea.Cmd) *Model {
	t.Helper()
	for i := 0; cmd != nil && i < 32; i++ {
		msg := cmd()
		if _, done := msg.(streamDoneMsg); done {
			model, _ := m.Update(msg)
			return model.(*Model)
		}
		model, next := m.Update(msg)
		m = model.(*Model)
		cmd = next
	}
	t.Fatal("token chain never produced streamDoneMsg")
	return nil
}

func TestQuit_DuringStreamCancelsDrainsThenCloses(t *testing.T) {
	repo := &fakeRepo{conv: &core.Conversation{ID: "conv-1"}}
	closer := &stubCloser{}
	stream := make(chan string, 8)
	brain := core.NewBrain(
		repo,
		&fakeProvider{events: []core.StreamEvent{{Text: "par"}, {Text: "tial"}}},
		core.WithSink(func(text string) { stream <- text }),
		core.WithSessionCloser(closer),
	)
	m := New(brain, repo, stream)
	m.input.SetValue("Hi")

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	// First token arrives; the reply is now streaming.
	msg := cmd()
	if _, ok := msg.(tokenMsg); !ok {
		t.Fatalf("first stream msg = %T, want tokenMsg", msg)
	}
	model, cmd = m.Update(msg)
	m = model.(*Model)

	// First CtrlC cancels the stream and shows the cancelling status.
	model, cancelCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)
	if !m.cancelled {
		t.Error("cancelled = false after first streaming CtrlC")
	}
	if !strings.Contains(m.View(), "cancelling") {
		t.Errorf("View() = %q, want a cancelling status line", m.View())
	}
	if cancelCmd != nil {
		t.Error("cancelCmd = non-nil, want nil (drain continues via waitToken)")
	}

	// The drain completes and commits the partial reply.
	m = driveUntilStreamDone(t, m, cmd)
	if m.streaming {
		t.Error("streaming = true after drain")
	}
	if !strings.Contains(m.history.String(), "assistant: partial") {
		t.Errorf("history = %q, want the committed partial reply", m.history.String())
	}

	// Now idle, the next CtrlC runs the bounded close and quits.
	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)
	if !m.closing {
		t.Fatal("closing = false for the post-drain quit")
	}
	closedMsgVal := cmd()
	model, cmd = m.Update(closedMsgVal)
	m = model.(*Model)
	if cmd == nil {
		t.Fatal("cmd = nil after closedMsg, want tea.Quit")
	}
	if closer.calls != 1 {
		t.Errorf("Close called %d times, want 1", closer.calls)
	}
}

func TestQuit_DuringStreamTwiceForceQuits(t *testing.T) {
	repo := &fakeRepo{}
	closer := &stubCloser{}
	stream := make(chan string, 8)
	brain := core.NewBrain(
		repo,
		&fakeProvider{events: []core.StreamEvent{{Text: "x"}}},
		core.WithSink(func(text string) { stream <- text }),
		core.WithSessionCloser(closer),
	)
	m := New(brain, repo, stream)
	m.input.SetValue("Hi")

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	// First press cancels; second press force-quits without closing.
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)

	model, quitCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)

	if quitCmd == nil {
		t.Fatal("cmd = nil, want an immediate quit command")
	}
	if _, ok := quitCmd().(tea.QuitMsg); !ok {
		t.Fatal("second streaming press did not force-quit with tea.QuitMsg")
	}
	if closer.calls != 0 {
		t.Errorf("Close called %d times, want 0 on force quit", closer.calls)
	}
	_ = cmd
	_ = m
}
