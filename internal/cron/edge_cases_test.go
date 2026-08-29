package cron_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/cron"
	"github.com/SalvucciFacundo/agis/internal/memory"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestEngine_ClosedEngine_AddJob(t *testing.T) {
	engine := cron.NewEngine()
	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}

	err := engine.AddJob(cron.Job{
		Name:     "job-closed",
		Schedule: "@every 1h",
		Prompt:   "ping",
	})
	if !errors.Is(err, cron.ErrSchedulerClosed) {
		t.Errorf("AddJob on closed engine error = %v, want ErrSchedulerClosed", err)
	}

	ctx := context.Background()
	err = engine.Start(ctx)
	if !errors.Is(err, cron.ErrSchedulerClosed) {
		t.Errorf("Start on closed engine error = %v, want ErrSchedulerClosed", err)
	}
}

func TestEngine_TargetSendError(t *testing.T) {
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
		replyText: "Sample output",
	}

	sender := &mockSender{
		sendErr: errors.New("network connection refused"),
	}

	engine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
		cron.WithEngineSender(sender),
	)

	err = engine.AddJob(cron.Job{
		Name:     "target-err-job",
		Schedule: "@every 15ms",
		Prompt:   "hello",
		Target: &cron.Target{
			Adapter:   "telegram",
			Recipient: "12345",
		},
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
	calls := len(sender.sendCalls)
	sender.mu.Unlock()

	if calls == 0 {
		t.Error("expected sender to be called even if it returned error")
	}
}

func TestEngine_NoBrainOrRepo_SafeHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Engine with no brain and no repo
	engine := cron.NewEngine()

	err := engine.AddJob(cron.Job{
		Name:     "nil-brain-job",
		Schedule: "@every 15ms",
		Prompt:   "hello",
	})
	if err != nil {
		t.Fatalf("AddJob error = %v", err)
	}

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("Start error = %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}
}

func TestEngine_JobsList(t *testing.T) {
	engine := cron.NewEngine()
	_ = engine.AddJob(cron.Job{Name: "j1", Schedule: "@every 1h", Prompt: "p1"})
	_ = engine.AddJob(cron.Job{Name: "j2", Schedule: "0 8 * * *", Prompt: "p2"})

	jobs := engine.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("len(Jobs()) = %d, want 2", len(jobs))
	}
	if jobs[0].Name != "j1" || jobs[1].Name != "j2" {
		t.Errorf("Jobs() returned unexpected list: %+v", jobs)
	}
}
