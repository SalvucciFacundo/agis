package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// tailLimit bounds how many recent messages Brain sends to the provider on
// each step. A positive value; the SQLite adapter treats limit <= 0 as
// "unbounded".
const tailLimit = 50

// Sink receives streamed text tokens from the provider.
type Sink func(text string)

// Brain orchestrates a single agent step: persist the user message, load the
// conversation tail, stream the provider's reply to the sink, and persist the
// assistant message. Tool calls are logged and ignored in M1.
type Brain struct {
	repo     Repository
	provider Provider
	sink     Sink
}

// Option configures a Brain.
type Option func(*Brain)

// WithSink sets the token sink. A nil sink discards streamed text.
func WithSink(s Sink) Option {
	return func(b *Brain) { b.sink = s }
}

// NewBrain returns a Brain backed by repo and provider.
func NewBrain(repo Repository, provider Provider, opts ...Option) *Brain {
	b := &Brain{repo: repo, provider: provider}
	for _, opt := range opts {
		opt(b)
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
