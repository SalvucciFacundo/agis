package gateway_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
)

type mockAdapter struct {
	name      string
	startErr  error
	stopErr   error
	sendErr   error
	started   atomic.Bool
	stopped   atomic.Bool
	sentMu    sync.Mutex
	sentCalls []struct {
		target string
		msg    string
	}
	handler gateway.Handler
}

func newMockAdapter(name string) *mockAdapter {
	return &mockAdapter{name: name}
}

func (m *mockAdapter) Name() string {
	return m.name
}

func (m *mockAdapter) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started.Store(true)
	return nil
}

func (m *mockAdapter) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped.Store(true)
	return nil
}

func (m *mockAdapter) Send(ctx context.Context, target string, msg string) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	m.sentCalls = append(m.sentCalls, struct {
		target string
		msg    string
	}{target: target, msg: msg})
	return nil
}

func (m *mockAdapter) SetHandler(h gateway.Handler) {
	m.handler = h
}

type mockBrain struct {
	mu           sync.Mutex
	activeConvID string
	stepCalls    []string
	repo         core.Repository
	replyText    string
}

func (b *mockBrain) SetActiveConversation(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeConvID = id
}

func (b *mockBrain) Step(ctx context.Context, input string) error {
	b.mu.Lock()
	b.stepCalls = append(b.stepCalls, input)
	convID := b.activeConvID
	b.mu.Unlock()

	// Append user and assistant messages to repo
	if b.repo != nil && convID != "" {
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleUser, Content: input})
		reply := b.replyText
		if reply == "" {
			reply = "Echo: " + input
		}
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleAssistant, Content: reply})
	}
	return nil
}

func TestMultiplexer_StartStop_MultipleAdapters(t *testing.T) {
	defer goleak.VerifyNone(t)

	tg := newMockAdapter("telegram")
	dc := newMockAdapter("discord")

	mux := gateway.NewMultiplexer()
	mux.RegisterAdapter(tg)
	mux.RegisterAdapter(dc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mux.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !tg.started.Load() || !dc.started.Load() {
		t.Errorf("adapters started = tg:%v, dc:%v, want both true", tg.started.Load(), dc.started.Load())
	}

	if err := mux.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if !tg.stopped.Load() || !dc.stopped.Load() {
		t.Errorf("adapters stopped = tg:%v, dc:%v, want both true", tg.stopped.Load(), dc.stopped.Load())
	}
}

func TestMultiplexer_Send(t *testing.T) {
	tg := newMockAdapter("telegram")
	dc := newMockAdapter("discord")

	mux := gateway.NewMultiplexer()
	mux.RegisterAdapter(tg)
	mux.RegisterAdapter(dc)

	ctx := context.Background()

	// Send to telegram
	if err := mux.Send(ctx, "telegram", "12345", "hello telegram"); err != nil {
		t.Fatalf("Send(telegram) error = %v", err)
	}
	tg.sentMu.Lock()
	if len(tg.sentCalls) != 1 || tg.sentCalls[0].target != "12345" || tg.sentCalls[0].msg != "hello telegram" {
		t.Errorf("tg sentCalls = %+v", tg.sentCalls)
	}
	tg.sentMu.Unlock()

	// Send to discord
	if err := mux.Send(ctx, "discord", "chan-99", "hello discord"); err != nil {
		t.Fatalf("Send(discord) error = %v", err)
	}
	dc.sentMu.Lock()
	if len(dc.sentCalls) != 1 || dc.sentCalls[0].target != "chan-99" || dc.sentCalls[0].msg != "hello discord" {
		t.Errorf("dc sentCalls = %+v", dc.sentCalls)
	}
	dc.sentMu.Unlock()

	// Send to unknown adapter
	if err := mux.Send(ctx, "slack", "user-1", "hello"); !errors.Is(err, gateway.ErrAdapterNotFound) {
		t.Errorf("Send(slack) error = %v, want %v", err, gateway.ErrAdapterNotFound)
	}
}

func TestMultiplexer_HandleEvent_SessionRoutingAndBrainExecution(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	repo, err := memory.NewRepository(ctx, ":memory:")
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:      repo,
		replyText: "Processed by Brain",
	}

	tg := newMockAdapter("telegram")
	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
	)
	mux.RegisterAdapter(tg)

	event := gateway.MessageEvent{
		Adapter:   "telegram",
		UserID:    "user-100",
		ChatID:    "chat-200",
		Content:   "Hello Brain",
		Timestamp: time.Now(),
	}

	if err := mux.HandleEvent(ctx, event); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}

	// Verify brain received step
	brain.mu.Lock()
	if len(brain.stepCalls) != 1 || brain.stepCalls[0] != "Hello Brain" {
		t.Errorf("brain stepCalls = %+v, want ['Hello Brain']", brain.stepCalls)
	}
	brain.mu.Unlock()

	// Verify reply was sent back to Telegram
	tg.sentMu.Lock()
	if len(tg.sentCalls) != 1 || tg.sentCalls[0].target != "chat-200" || tg.sentCalls[0].msg != "Processed by Brain" {
		t.Errorf("tg sentCalls = %+v, want reply 'Processed by Brain'", tg.sentCalls)
	}
	tg.sentMu.Unlock()

	// Verify session continuity on second message
	event2 := gateway.MessageEvent{
		Adapter:   "telegram",
		UserID:    "user-100",
		ChatID:    "chat-200",
		Content:   "Second message",
		Timestamp: time.Now(),
	}
	if err := mux.HandleEvent(ctx, event2); err != nil {
		t.Fatalf("HandleEvent(2) error = %v", err)
	}

	// Verify messages in DB
	convID := mux.GetSessionConvID("gateway:telegram:chat-200")
	if convID == "" {
		t.Fatal("session conv ID is empty")
	}

	msgs, err := repo.Messages(ctx, convID, 10)
	if err != nil {
		t.Fatalf("repo.Messages error = %v", err)
	}
	if len(msgs) != 4 { // 2 user turns + 2 assistant replies
		t.Errorf("len(msgs) = %d, want 4", len(msgs))
	}
}
