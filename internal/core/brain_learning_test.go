package core

import (
	"context"
	"log/slog"
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
