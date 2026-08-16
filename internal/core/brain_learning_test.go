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

