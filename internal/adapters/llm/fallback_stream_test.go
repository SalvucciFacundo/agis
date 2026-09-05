package llm

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"go.uber.org/goleak"
)

func TestFallbackProvider_Stream_PreTokenFailover(t *testing.T) {
	defer goleak.VerifyNone(t)

	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			primaryCalled.Add(1)
			// Return an immediate transient error before stream starts
			return nil, fmt.Errorf("chat completion: Service Unavailable (status 503)")
		},
	}

	fallback := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			fallbackCalled.Add(1)
			ch := make(chan core.StreamEvent, 2)
			ch <- core.StreamEvent{Text: "Hello from "}
			ch <- core.StreamEvent{Text: "fallback"}
			close(ch)
			return ch, nil
		},
	}

	fb := NewFallbackProvider(primary, fallback)
	ch, err := fb.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v, want nil", err)
	}

	var sb strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("unexpected stream error event: %v", ev.Err)
		}
		sb.WriteString(ev.Text)
	}

	if sb.String() != "Hello from fallback" {
		t.Errorf("stream output = %q, want 'Hello from fallback'", sb.String())
	}
	if primaryCalled.Load() != 1 {
		t.Errorf("primary called %d times, want 1", primaryCalled.Load())
	}
	if fallbackCalled.Load() != 1 {
		t.Errorf("fallback called %d times, want 1", fallbackCalled.Load())
	}
}

func TestFallbackProvider_Stream_PreTokenChannelErrorFailover(t *testing.T) {
	defer goleak.VerifyNone(t)

	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			primaryCalled.Add(1)
			ch := make(chan core.StreamEvent, 1)
			// Emits transient error without any preceding tokens
			ch <- core.StreamEvent{Err: fmt.Errorf("status 502 Bad Gateway")}
			close(ch)
			return ch, nil
		},
	}

	fallback := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			fallbackCalled.Add(1)
			ch := make(chan core.StreamEvent, 1)
			ch <- core.StreamEvent{Text: "recovered"}
			close(ch)
			return ch, nil
		},
	}

	fb := NewFallbackProvider(primary, fallback)
	ch, err := fb.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v, want nil", err)
	}

	var sb strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("unexpected stream error event: %v", ev.Err)
		}
		sb.WriteString(ev.Text)
	}

	if sb.String() != "recovered" {
		t.Errorf("stream output = %q, want 'recovered'", sb.String())
	}
	if primaryCalled.Load() != 1 {
		t.Errorf("primary called %d times, want 1", primaryCalled.Load())
	}
	if fallbackCalled.Load() != 1 {
		t.Errorf("fallback called %d times, want 1", fallbackCalled.Load())
	}
}

func TestFallbackProvider_Stream_MidStreamErrorTerminates(t *testing.T) {
	defer goleak.VerifyNone(t)

	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			primaryCalled.Add(1)
			ch := make(chan core.StreamEvent, 3)
			ch <- core.StreamEvent{Text: "prefix token "}
			ch <- core.StreamEvent{Err: fmt.Errorf("connection reset by peer (status 500)")}
			close(ch)
			return ch, nil
		},
	}

	fallback := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			fallbackCalled.Add(1)
			ch := make(chan core.StreamEvent, 1)
			ch <- core.StreamEvent{Text: "should not be called"}
			close(ch)
			return ch, nil
		},
	}

	fb := NewFallbackProvider(primary, fallback)
	ch, err := fb.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v, want nil", err)
	}

	var events []core.StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (text event then error event)", len(events))
	}
	if events[0].Text != "prefix token " {
		t.Errorf("events[0].Text = %q, want 'prefix token '", events[0].Text)
	}
	if events[1].Err == nil {
		t.Fatal("events[1].Err = nil, want mid-stream error event")
	}
	if !strings.Contains(events[1].Err.Error(), "connection reset") {
		t.Errorf("events[1].Err = %v, want connection reset", events[1].Err)
	}
	if fallbackCalled.Load() != 0 {
		t.Errorf("fallback called %d times, want 0 (mid-stream failure must not failover)", fallbackCalled.Load())
	}
}

func TestFallbackProvider_Stream_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	primary := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			ch := make(chan core.StreamEvent)
			go func() {
				defer close(ch)
				<-ctx.Done()
			}()
			return ch, nil
		},
	}

	fb := NewFallbackProvider(primary)
	ch, err := fb.Stream(ctx, core.ChatRequest{})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	cancel()

	// Drain channel
	for range ch {
	}
}

func TestFallbackProvider_Stream_AllProvidersFailPreToken(t *testing.T) {
	defer goleak.VerifyNone(t)

	primary := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			return nil, fmt.Errorf("status 503 Service Unavailable")
		},
	}
	fallback := &mockProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			return nil, fmt.Errorf("status 502 Bad Gateway")
		},
	}

	fb := NewFallbackProvider(primary, fallback)
	ch, err := fb.Stream(context.Background(), core.ChatRequest{})
	if err != nil {
		// Even if returned immediately or via channel, must be an error
		if !strings.Contains(err.Error(), "all LLM providers failed") {
			t.Errorf("Stream error = %v, want 'all LLM providers failed'", err)
		}
		return
	}

	var errReceived error
	for ev := range ch {
		if ev.Err != nil {
			errReceived = ev.Err
		}
	}
	if errReceived == nil {
		t.Fatal("expected error event on channel when all providers fail pre-token")
	}
	if !strings.Contains(errReceived.Error(), "all LLM providers failed") {
		t.Errorf("errReceived = %v, want 'all LLM providers failed'", errReceived)
	}
}
