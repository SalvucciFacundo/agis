package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// TestNewBrain_LearningDefaults proves the learning-loop knobs default to the
// spec values when no options are supplied, and that a nil logger is replaced
// with the default logger.
func TestNewBrain_LearningDefaults(t *testing.T) {
	b := NewBrain(newFakeRepo(), &fakeProvider{})

	if b.recallLimit != 10 {
		t.Errorf("recallLimit = %d, want 10", b.recallLimit)
	}
	if b.nudgeEvery != 10 {
		t.Errorf("nudgeEvery = %d, want 10", b.nudgeEvery)
	}
	if b.nudger != nil {
		t.Error("nudger = non-nil, want nil (disabled by default)")
	}
	if b.closer != nil {
		t.Error("closer = non-nil, want nil (disabled by default)")
	}
	if b.logger == nil {
		t.Error("logger = nil, want the default logger")
	}
}

// TestNewBrain_LearningOptions proves each learning option overrides its
// default and that a non-positive recall limit falls back to the default.
func TestNewBrain_LearningOptions(t *testing.T) {
	nudger := &stubNudger{}
	closer := &stubCloser{}
	logger := slog.New(slog.DiscardHandler)

	b := NewBrain(
		newFakeRepo(),
		&fakeProvider{},
		WithNudger(nudger),
		WithSessionCloser(closer),
		WithLogger(logger),
		WithRecallLimit(5),
		WithNudgeEvery(3),
	)

	if b.nudger != nudger {
		t.Errorf("nudger not wired")
	}
	if b.closer != closer {
		t.Errorf("closer not wired")
	}
	if b.logger != logger {
		t.Errorf("logger not wired")
	}
	if b.recallLimit != 5 {
		t.Errorf("recallLimit = %d, want 5", b.recallLimit)
	}
	if b.nudgeEvery != 3 {
		t.Errorf("nudgeEvery = %d, want 3", b.nudgeEvery)
	}

	zero := NewBrain(newFakeRepo(), &fakeProvider{}, WithRecallLimit(0))
	if zero.recallLimit != 10 {
		t.Errorf("recallLimit(0) = %d, want fallback 10", zero.recallLimit)
	}
}

// stubNudger and stubCloser are minimal Nudger/SessionCloser doubles used to
// prove option wiring compiles against the consumer-side interfaces.
type stubNudger struct{}

func (stubNudger) Nudge(context.Context, string, []Message) ([]Observation, error) {
	return nil, nil
}

type stubCloser struct{}

func (stubCloser) Close(context.Context, string, []Message) error { return nil }

// capturingProvider records the ChatRequest passed to Stream so recall tests
// can assert what the provider saw.
type capturingProvider struct {
	events   []StreamEvent
	requests []ChatRequest
}

var _ Provider = (*capturingProvider)(nil)

func (c *capturingProvider) Chat(context.Context, ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("Chat not used in brain tests")
}

func (c *capturingProvider) Stream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	c.requests = append(c.requests, req)
	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		for _, ev := range c.events {
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (c *capturingProvider) Models() []ModelInfo { return nil }

func TestBrainStep_InjectsRecall(t *testing.T) {
	repo := newFakeRepo()
	repo.observations = []Observation{
		{TopicKey: "user/pref/coffee", Content: "dark roast", Importance: 4},
	}
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider)

	if err := brain.Step(context.Background(), "hello"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(provider.requests) != 1 {
		t.Fatalf("Stream called %d times, want 1", len(provider.requests))
	}
	msgs := provider.requests[0].Messages
	if len(msgs) == 0 || msgs[0].Role != RoleSystem {
		t.Fatalf("first message = %+v, want the system recall message", msgs)
	}
	if !strings.Contains(msgs[0].Content, "Relevant memories:") {
		t.Errorf("recall message = %q, want the recall header", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "dark roast") {
		t.Errorf("recall message = %q, want the observation content", msgs[0].Content)
	}
	// Default recall limit reaches the repository read.
	if repo.lastRecallLimit != 10 {
		t.Errorf("Observations called with limit %d, want 10", repo.lastRecallLimit)
	}
}

func TestBrainStep_NoRecallWhenEmpty(t *testing.T) {
	repo := newFakeRepo()
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider)

	if err := brain.Step(context.Background(), "hello"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(provider.requests) != 1 {
		t.Fatalf("Stream called %d times, want 1", len(provider.requests))
	}
	msgs := provider.requests[0].Messages
	if len(msgs) != 1 || msgs[0].Role != RoleUser {
		t.Errorf("messages = %+v, want just the user message (no recall)", msgs)
	}
}

// fakeNudger records Nudge invocations and returns a configurable error.
type fakeNudger struct {
	calls int
	err   error
}

var _ Nudger = (*fakeNudger)(nil)

func (f *fakeNudger) Nudge(context.Context, string, []Message) ([]Observation, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func TestBrainStep_NudgesOnBoundary(t *testing.T) {
	repo := newFakeRepo()
	nudger := &fakeNudger{}
	provider := &fakeProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider, WithNudger(nudger), WithNudgeEvery(2))

	for i := 0; i < 4; i++ {
		if err := brain.Step(context.Background(), "hi"); err != nil {
			t.Fatalf("Step(%d) error = %v", i, err)
		}
	}

	// 4 assistant messages, nudgeEvery=2 → 2 nudges.
	if nudger.calls != 2 {
		t.Errorf("Nudge called %d times, want 2", nudger.calls)
	}
	if len(repo.sessionEvents) != 2 {
		t.Fatalf("got %d session events, want 2", len(repo.sessionEvents))
	}
	for _, e := range repo.sessionEvents {
		if e.kind != "nudge" {
			t.Errorf("session event kind = %q, want nudge", e.kind)
		}
	}
}

func TestBrainStep_NudgeDisabledWhenNil(t *testing.T) {
	repo := newFakeRepo()
	provider := &fakeProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider, WithNudgeEvery(1)) // no nudger wired

	for i := 0; i < 3; i++ {
		if err := brain.Step(context.Background(), "hi"); err != nil {
			t.Fatalf("Step(%d) error = %v", i, err)
		}
	}
	if len(repo.sessionEvents) != 0 {
		t.Errorf("got %d session events, want 0 (nil curator disables nudge)", len(repo.sessionEvents))
	}
}

func TestBrainStep_NudgeEveryZeroDisables(t *testing.T) {
	repo := newFakeRepo()
	nudger := &fakeNudger{}
	provider := &fakeProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider, WithNudger(nudger), WithNudgeEvery(0))

	for i := 0; i < 5; i++ {
		if err := brain.Step(context.Background(), "hi"); err != nil {
			t.Fatalf("Step(%d) error = %v", i, err)
		}
	}
	if nudger.calls != 0 {
		t.Errorf("Nudge called %d times, want 0 (nudgeEvery=0 disables)", nudger.calls)
	}
}

func TestBrainStep_NudgeErrorNonFatal(t *testing.T) {
	repo := newFakeRepo()
	nudger := &fakeNudger{err: errors.New("curator down")}
	provider := &fakeProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(
		repo,
		provider,
		WithNudger(nudger),
		WithNudgeEvery(1),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	if err := brain.Step(context.Background(), "hi"); err != nil {
		t.Fatalf("Step() error = %v, want nil (nudge error is non-fatal)", err)
	}
	msgs, _ := repo.Messages(context.Background(), "conv-1", 0)
	if len(msgs) != 2 {
		t.Errorf("got %d messages, want 2 (turn completed despite nudge failure)", len(msgs))
	}
}

// fakeCloser records Close invocations and returns a configurable error.
type fakeCloser struct {
	calls    int
	err      error
	convID   string
	msgCount int
}

var _ SessionCloser = (*fakeCloser)(nil)

func (f *fakeCloser) Close(_ context.Context, convID string, msgs []Message) error {
	f.calls++
	f.convID = convID
	f.msgCount = len(msgs)
	return f.err
}

func TestBrainCloseSession_Orchestrates(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{}
	brain := NewBrain(repo, &fakeProvider{}, WithSessionCloser(closer))

	// Seed a conversation with a message.
	if _, err := repo.CreateConversation(context.Background(), ""); err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := repo.AppendMessage(context.Background(), "conv-1", Message{Role: RoleUser, Content: "hi"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}

	if closer.calls != 1 {
		t.Fatalf("Close called %d times, want 1", closer.calls)
	}
	if closer.convID != "conv-1" {
		t.Errorf("Close convID = %q, want conv-1", closer.convID)
	}
	if closer.msgCount != 1 {
		t.Errorf("Close got %d messages, want 1", closer.msgCount)
	}
	if len(repo.sessionEvents) != 1 || repo.sessionEvents[0].kind != "summary" {
		t.Errorf("session events = %+v, want one summary event", repo.sessionEvents)
	}
}

func TestBrainCloseSession_NilCloserNoop(t *testing.T) {
	repo := newFakeRepo()
	brain := NewBrain(repo, &fakeProvider{}) // no closer wired

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v, want nil", err)
	}
	if len(repo.sessionEvents) != 0 {
		t.Errorf("session events = %v, want none (nil closer is a no-op)", repo.sessionEvents)
	}
}

func TestBrainCloseSession_NoConversationNoop(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{}
	brain := NewBrain(repo, &fakeProvider{}, WithSessionCloser(closer))

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if closer.calls != 0 {
		t.Errorf("Close called %d times, want 0 (no conversation to close)", closer.calls)
	}
}

func TestBrainCloseSession_ErrorNonFatal(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{err: errors.New("summarizer down")}
	brain := NewBrain(
		repo,
		&fakeProvider{},
		WithSessionCloser(closer),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	if _, err := repo.CreateConversation(context.Background(), ""); err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v, want nil (non-fatal)", err)
	}
	// No summary event because Close failed before producing a summary.
	if len(repo.sessionEvents) != 0 {
		t.Errorf("session events = %v, want none after a failed close", repo.sessionEvents)
	}
}
