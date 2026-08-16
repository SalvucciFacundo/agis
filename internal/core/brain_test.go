package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fakeProvider is a test double for the Provider port.
type fakeProvider struct {
	events    []StreamEvent
	streamErr error
}

var _ Provider = (*fakeProvider)(nil)

func (f *fakeProvider) Chat(context.Context, ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("Chat not used in brain tests")
}

func (f *fakeProvider) Stream(ctx context.Context, _ ChatRequest) (<-chan StreamEvent, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	ch := make(chan StreamEvent)
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

func (f *fakeProvider) Models() []ModelInfo {
	return nil
}

// fakeRepo is an in-memory test double for the Repository port.
type fakeRepo struct {
	mu           sync.Mutex
	convs        map[string]*Conversation
	messages     map[string][]Message
	observations []Observation
	latest       *Conversation
}

var _ Repository = (*fakeRepo)(nil)

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		convs:    map[string]*Conversation{},
		messages: map[string][]Message{},
	}
}

func (r *fakeRepo) CreateConversation(_ context.Context, title string) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conv := &Conversation{ID: "conv-1", Title: title}
	r.convs[conv.ID] = conv
	r.latest = conv
	return conv, nil
}

func (r *fakeRepo) LatestConversation(_ context.Context) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil, ErrNotFound
	}
	return r.latest, nil
}

func (r *fakeRepo) AppendMessage(_ context.Context, convID string, msg Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[convID] = append(r.messages[convID], msg)
	return nil
}

func (r *fakeRepo) Messages(_ context.Context, convID string, limit int) ([]Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	msgs := r.messages[convID]
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (r *fakeRepo) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	return []SearchResult{}, nil
}

func (r *fakeRepo) SaveObservations(_ context.Context, _ string, obs []Observation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observations = append(r.observations, obs...)
	return nil
}

func (r *fakeRepo) Observations(_ context.Context, _ int) ([]Observation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Observation, len(r.observations))
	copy(out, r.observations)
	return out, nil
}

func (r *fakeRepo) UpdateConversationSummary(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *fakeRepo) UpsertUserModel(_ context.Context, _ []UserModel) error {
	return nil
}

func (r *fakeRepo) Close() error {
	return nil
}

func TestBrainStep_StreamsAndPersists(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{events: []StreamEvent{{Text: "Hi"}}}

	var sink strings.Builder
	brain := NewBrain(repo, provider, WithSink(func(s string) { sink.WriteString(s) }))

	if err := brain.Step(context.Background(), "Hello"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if got := sink.String(); got != "Hi" {
		t.Errorf("sink got %q, want %q", got, "Hi")
	}

	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != RoleUser || msgs[0].Content != "Hello" {
		t.Errorf("message[0] = %+v, want user/Hello", msgs[0])
	}
	if msgs[1].Role != RoleAssistant || msgs[1].Content != "Hi" {
		t.Errorf("message[1] = %+v, want assistant/Hi", msgs[1])
	}
}

func TestBrainStep_AccumulatesTokens(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{events: []StreamEvent{{Text: "Hel"}, {Text: "lo"}}}

	var sink strings.Builder
	brain := NewBrain(repo, provider, WithSink(func(s string) { sink.WriteString(s) }))

	if err := brain.Step(context.Background(), "Hi"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[1].Content != "Hello" {
		t.Errorf("assistant content = %q, want %q", msgs[1].Content, "Hello")
	}
	if sink.String() != "Hello" {
		t.Errorf("sink got %q, want %q", sink.String(), "Hello")
	}
}

func TestBrainStep_ImmediateStreamError(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{streamErr: errors.New("boom")}
	brain := NewBrain(repo, provider)

	if err := brain.Step(context.Background(), "Hello"); err == nil {
		t.Fatal("Step() error = nil, want error")
	}

	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (user only)", len(msgs))
	}
	if msgs[0].Role != RoleUser {
		t.Errorf("message role = %q, want user", msgs[0].Role)
	}
}

func TestBrainStep_MidStreamError(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{events: []StreamEvent{
		{Text: "Hi"},
		{Err: errors.New("mid-stream boom")},
	}}
	brain := NewBrain(repo, provider)

	if err := brain.Step(context.Background(), "Hello"); err == nil {
		t.Fatal("Step() error = nil, want error")
	}

	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (user only)", len(msgs))
	}
	if msgs[0].Role != RoleUser {
		t.Errorf("message role = %q, want user", msgs[0].Role)
	}
}

func TestBrainStep_ReusesLatestConversation(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{events: []StreamEvent{{Text: "x"}}}
	brain := NewBrain(repo, provider)

	if err := brain.Step(context.Background(), "one"); err != nil {
		t.Fatalf("Step(one) error = %v", err)
	}
	if err := brain.Step(context.Background(), "two"); err != nil {
		t.Fatalf("Step(two) error = %v", err)
	}

	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 4 {
		t.Fatalf("got %d messages, want 4 across one conversation", len(msgs))
	}
}
