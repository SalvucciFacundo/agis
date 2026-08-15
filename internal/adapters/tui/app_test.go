package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

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
