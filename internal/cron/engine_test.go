package cron_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/cron"
	"github.com/SalvucciFacundo/agis/internal/memory"
)

type mockBrain struct {
	mu           sync.Mutex
	activeConvID string
	stepCalls    []string
	stepErr      error
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
	err := b.stepErr
	b.mu.Unlock()

	if err != nil {
		return err
	}

	if b.repo != nil && convID != "" {
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleUser, Content: input})
		reply := b.replyText
		if reply == "" {
			reply = "Done: " + input
		}
		_ = b.repo.AppendMessage(ctx, convID, core.Message{Role: core.RoleAssistant, Content: reply})
	}
	return nil
}

type mockSender struct {
	mu        sync.Mutex
	sendCalls []struct {
		adapter string
		target  string
		msg     string
	}
	sendErr error
}

func (s *mockSender) Send(ctx context.Context, adapter, target, msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendCalls = append(s.sendCalls, struct {
		adapter string
		target  string
		msg     string
	}{adapter: adapter, target: target, msg: msg})
	return s.sendErr
}

func TestEngine_StartStop_GracefulShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	engine := cron.NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Double start should return error or be idempotent
	if err := engine.Start(ctx); err == nil {
		t.Error("Start() expected error when already started")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Double stop should be a safe no-op
	if err := engine.Stop(); err != nil {
		t.Errorf("second Stop() error = %v", err)
	}
}

func TestEngine_AddJob_Validation(t *testing.T) {
	defer goleak.VerifyNone(t)

	engine := cron.NewEngine()

	// Valid job
	err := engine.AddJob(cron.Job{
		Name:     "job-1",
		Schedule: "@every 10s",
		Prompt:   "ping",
	})
	if err != nil {
		t.Fatalf("AddJob() valid error = %v", err)
	}

	// Invalid job (missing name)
	err = engine.AddJob(cron.Job{
		Schedule: "@every 10s",
		Prompt:   "ping",
	})
	if err == nil {
		t.Error("AddJob() expected error on missing name")
	}

	// Invalid job (bad schedule)
	err = engine.AddJob(cron.Job{
		Name:     "job-2",
		Schedule: "bad schedule",
		Prompt:   "ping",
	})
	if err == nil {
		t.Error("AddJob() expected error on bad schedule")
	}
}

func TestEngine_TriggerExecution_EphemeralSession(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:      repo,
		replyText: "Summary of system status: all good",
	}

	sender := &mockSender{}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
		cron.WithEngineSender(sender),
	)

	// Add job with @every 20ms
	err = engine.AddJob(cron.Job{
		Name:     "daily-health",
		Schedule: "@every 20ms",
		Prompt:   "Check system status",
		Target: &cron.Target{
			Adapter:   "telegram",
			Recipient: "998877",
		},
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	// Wait for at least 1 run
	time.Sleep(60 * time.Millisecond)

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	brain.mu.Lock()
	stepCount := len(brain.stepCalls)
	brain.mu.Unlock()

	if stepCount == 0 {
		t.Fatal("expected at least 1 brain step execution, got 0")
	}

	// Verify session name in conversation
	convs, err := repo.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations error = %v", err)
	}
	var foundEphemeral bool
	for _, c := range convs {
		if c.Title == "cron:daily-health" {
			foundEphemeral = true
			break
		}
	}
	if !foundEphemeral {
		t.Errorf("expected conversation with title 'cron:daily-health', got %v", convs)
	}

	// Verify sender received target notification
	sender.mu.Lock()
	var foundCall bool
	for _, call := range sender.sendCalls {
		if call.adapter == "telegram" && call.target == "998877" && call.msg == "Summary of system status: all good" {
			foundCall = true
			break
		}
	}
	sendCallsCopy := make([]struct {
		adapter string
		target  string
		msg     string
	}, len(sender.sendCalls))
	copy(sendCallsCopy, sender.sendCalls)
	sender.mu.Unlock()

	if !foundCall {
		t.Errorf("expected target notification to be delivered, got calls: %+v", sendCallsCopy)
	}
}

func TestEngine_TriggerExecution_BoundSession(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:      repo,
		replyText: "Custom session output",
	}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
	)

	err = engine.AddJob(cron.Job{
		Name:      "custom-job",
		Schedule:  "@every 20ms",
		Prompt:    "Run custom job",
		SessionID: "my-persistent-session",
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	convs, err := repo.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations error = %v", err)
	}
	var foundBound bool
	for _, c := range convs {
		if c.Title == "my-persistent-session" {
			foundBound = true
			break
		}
	}
	if !foundBound {
		t.Errorf("expected conversation with title 'my-persistent-session', got %v", convs)
	}
}

func TestEngine_NoTarget_LogsOnly(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:      repo,
		replyText: "Output without target",
	}

	sender := &mockSender{}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
		cron.WithEngineSender(sender),
	)

	err = engine.AddJob(cron.Job{
		Name:     "log-only-job",
		Schedule: "@every 20ms",
		Prompt:   "Log something",
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	sender.mu.Lock()
	sendCount := len(sender.sendCalls)
	sender.mu.Unlock()

	if sendCount != 0 {
		t.Errorf("expected 0 sender calls when no target configured, got %d", sendCount)
	}
}

func TestEngine_BrainError_LoggedGracefully(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo:    repo,
		stepErr: errors.New("brain provider timeout"),
	}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
	)

	err = engine.AddJob(cron.Job{
		Name:     "failing-job",
		Schedule: "@every 20ms",
		Prompt:   "Will fail",
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Stop must succeed cleanly even if jobs returned errors
	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}
}

func TestEngine_ConcurrentJobs(t *testing.T) {
	defer goleak.VerifyNone(t)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := memory.NewRepository(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewRepository error = %v", err)
	}
	defer repo.Close()

	brain := &mockBrain{
		repo: repo,
	}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
	)

	for i := 0; i < 5; i++ {
		err := engine.AddJob(cron.Job{
			Name:     string(rune('A' + i)),
			Schedule: "@every 15ms",
			Prompt:   "Concurrent job prompt",
		})
		if err != nil {
			t.Fatalf("AddJob error = %v", err)
		}
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	var count int
	for i := 0; i < 100; i++ {
		brain.mu.Lock()
		count = len(brain.stepCalls)
		brain.mu.Unlock()
		if count >= 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	if count < 5 {
		t.Errorf("expected at least 5 step calls across 5 concurrent jobs, got %d", count)
	}
}
