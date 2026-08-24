package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// tailLimit bounds how many recent messages Brain sends to the provider on
// each step. A positive value; the SQLite adapter treats limit <= 0 as
// "unbounded".
const tailLimit = 50

// closeMessageLimit bounds how many messages CloseSession hands to the
// summarizer.
const closeMessageLimit = 200

// Defaults for the learning-loop knobs, overridable via options and, in a
// later milestone, configuration.
const (
	defaultRecallLimit = 10
	defaultNudgeEvery  = 10
)

// Sink receives streamed text tokens from the provider.
type Sink func(text string)

// Nudger is the consumer-side interface for the learning-loop curator. It is
// satisfied by *memory.Curator; defining it here (rather than depending on
// that package) keeps core free of an import cycle. A nil Nudger disables
// periodic curation.
type Nudger interface {
	Nudge(ctx context.Context, convID string, msgs []Message) ([]Observation, error)
}

// SessionCloser is the consumer-side interface for the learning-loop
// summarizer. It is satisfied by *memory.Summarizer. A nil SessionCloser
// disables close-time summarization.
type SessionCloser interface {
	Close(ctx context.Context, convID string, msgs []Message) error
}

// Brain orchestrates a single agent step: persist the user message, load the
// conversation tail, stream the provider's reply to the sink, and persist the
// assistant message. Tool calls are logged and ignored in M1.
type Brain struct {
	repo     Repository
	provider Provider
	sink     Sink

	nudger Nudger
	closer SessionCloser
	logger *slog.Logger

	// recallLimit bounds how many observations Step injects into the system
	// prompt (default 10).
	recallLimit int
	// nudgeEvery triggers a curation nudge every N assistant messages. Zero
	// disables nudging (default 10).
	nudgeEvery int
	// assistantCount counts completed assistant messages in this Brain's
	// lifetime; it is the nudge cadence counter.
	assistantCount int
}

// Option configures a Brain.
type Option func(*Brain)

// WithSink sets the token sink. A nil sink discards streamed text.
func WithSink(s Sink) Option {
	return func(b *Brain) { b.sink = s }
}

// WithNudger wires the curator. A nil Nudger (the default) disables nudging.
func WithNudger(n Nudger) Option {
	return func(b *Brain) { b.nudger = n }
}

// WithSessionCloser wires the summarizer. A nil SessionCloser (the default)
// disables close-time summarization.
func WithSessionCloser(c SessionCloser) Option {
	return func(b *Brain) { b.closer = c }
}

// WithLogger sets the structured logger used for non-fatal learning-loop
// warnings. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(b *Brain) { b.logger = l }
}

// WithRecallLimit sets the top-N observation recall bound. A non-positive
// value falls back to the default of 10.
func WithRecallLimit(n int) Option {
	return func(b *Brain) { b.recallLimit = n }
}

// WithNudgeEvery sets the nudge cadence. Zero disables nudging.
func WithNudgeEvery(n int) Option {
	return func(b *Brain) { b.nudgeEvery = n }
}

// NewBrain returns a Brain backed by repo and provider.
func NewBrain(repo Repository, provider Provider, opts ...Option) *Brain {
	b := &Brain{
		repo:        repo,
		provider:    provider,
		logger:      slog.Default(),
		recallLimit: defaultRecallLimit,
		nudgeEvery:  defaultNudgeEvery,
	}
	for _, opt := range opts {
		opt(b)
	}
	if b.recallLimit <= 0 {
		b.recallLimit = defaultRecallLimit
	}
	return b
}

// Step processes one user turn and returns an error on failure. On a
// provider error the user message remains persisted and no assistant message
// is written.
func (b *Brain) Step(ctx context.Context, input string) error {
	conv, err := b.ensureConversation(ctx)
	if err != nil {
		return err
	}

	if err := b.repo.AppendMessage(ctx, conv.ID, Message{Role: RoleUser, Content: input}); err != nil {
		return fmt.Errorf("persisting user message: %w", err)
	}

	tail, err := b.repo.Messages(ctx, conv.ID, tailLimit)
	if err != nil {
		return fmt.Errorf("loading conversation tail: %w", err)
	}

	events, err := b.provider.Stream(ctx, ChatRequest{Messages: tail})
	if err != nil {
		return fmt.Errorf("streaming response: %w", err)
	}

	var reply strings.Builder
	for ev := range events {
		if ev.Err != nil {
			// Drain the channel to its close before returning: the provider
			// goroutine may still be blocked sending on an unbuffered channel,
			// and the port contract requires providers to close the channel
			// after a terminal Err event.
			for range events {
			}
			return fmt.Errorf("stream error: %w", ev.Err)
		}
		reply.WriteString(ev.Text)
		if b.sink != nil {
			b.sink(ev.Text)
		}
	}

	if err := b.repo.AppendMessage(ctx, conv.ID, Message{Role: RoleAssistant, Content: reply.String()}); err != nil {
		return fmt.Errorf("persisting assistant message: %w", err)
	}

	return nil
}

// ensureConversation returns the latest conversation, creating one when none
// exists yet.
func (b *Brain) ensureConversation(ctx context.Context) (*Conversation, error) {
	conv, err := b.repo.LatestConversation(ctx)
	if err == nil {
		return conv, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("loading latest conversation: %w", err)
	}
	return b.repo.CreateConversation(ctx, "")
}
