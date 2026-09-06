package subagents_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/subagents"
	"go.uber.org/goleak"
)

type testProvider struct {
	mu          sync.Mutex
	streamFunc  func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error)
	streamDelay time.Duration
	replyText   string
}

func (p *testProvider) Chat(context.Context, core.ChatRequest) (core.ChatResponse, error) {
	return core.ChatResponse{}, errors.New("chat not used")
}

func (p *testProvider) Models() []core.ModelInfo { return nil }

func (p *testProvider) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	if p.streamFunc != nil {
		return p.streamFunc(ctx, req)
	}
	ch := make(chan core.StreamEvent)
	go func() {
		defer close(ch)
		if p.streamDelay > 0 {
			select {
			case <-time.After(p.streamDelay):
			case <-ctx.Done():
				return
			}
		}
		select {
		case ch <- core.StreamEvent{Text: p.replyText}:
		case <-ctx.Done():
		}
	}()
	return ch, nil
}

type testGuard struct {
	decision core.Decision
}

func (g *testGuard) Evaluate(context.Context, core.GuardRequest) core.Decision {
	return g.decision
}

func TestEngine_SuccessfulSpawnAndSynthesis(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	prov := &testProvider{replyText: "Synthesized child result"}
	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  3,
		MaxDepth:       1,
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       8,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	res, err := engine.Spawn(ctx, "Analyze data", "Context details", 8)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if res != "Synthesized child result" {
		t.Errorf("Spawn result = %q, want 'Synthesized child result'", res)
	}
}

func TestEngine_ValidationAndDisabled(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	prov := &testProvider{replyText: "ok"}

	// Disabled engine
	cfgDisabled := config.SubagentsConfig{Enabled: false, MaxConcurrent: 3, MaxDepth: 1}
	engineDisabled := subagents.NewEngine(cfgDisabled, parent, prov, nil, nil, nil)
	_, err := engineDisabled.Spawn(ctx, "task", "", 8)
	if err == nil || !strings.Contains(err.Error(), "disabled by configuration") {
		t.Fatalf("expected disabled error, got: %v", err)
	}

	// Enabled engine with empty task
	cfgEnabled := config.SubagentsConfig{Enabled: true, MaxConcurrent: 3, MaxDepth: 1}
	engine := subagents.NewEngine(cfgEnabled, parent, prov, nil, nil, nil)
	_, err = engine.Spawn(ctx, "   ", "", 8)
	if err == nil || !strings.Contains(err.Error(), "task parameter is required and cannot be empty") {
		t.Fatalf("expected empty task error, got: %v", err)
	}
}

func TestEngine_ConcurrencyLimitWithSemaphore(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()

	var running atomic.Int32
	var maxObserved atomic.Int32

	prov := &testProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			ch := make(chan core.StreamEvent)
			curr := running.Add(1)
			for {
				old := maxObserved.Load()
				if curr > old {
					if maxObserved.CompareAndSwap(old, curr) {
						break
					}
				} else {
					break
				}
			}

			go func() {
				defer close(ch)
				defer running.Add(-1)
				select {
				case <-time.After(50 * time.Millisecond):
					select {
					case ch <- core.StreamEvent{Text: "done"}:
					case <-ctx.Done():
					}
				case <-ctx.Done():
				}
			}()
			return ch, nil
		},
	}

	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2, // max 2 concurrent
		MaxDepth:       1,
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       8,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.Spawn(ctx, "concurrent task", "", 8)
		}()
	}
	wg.Wait()

	if maxObserved.Load() > 2 {
		t.Errorf("max concurrent observed was %d, expected <= 2", maxObserved.Load())
	}
}

func TestEngine_TimeoutPropagationAndCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	parent := newFakeParentRepo()
	prov := &testProvider{
		streamDelay: 500 * time.Millisecond,
		replyText:   "slow response",
	}

	// 1. Engine default timeout triggers
	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2,
		MaxDepth:       1,
		DefaultTimeout: 50 * time.Millisecond, // very short timeout
		MaxTurns:       8,
	}
	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	_, err := engine.Spawn(context.Background(), "slow task", "", 8)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	// 2. Parent context cancellation cancels child immediately
	cfgLong := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2,
		MaxDepth:       1,
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       8,
	}
	engineLong := subagents.NewEngine(cfgLong, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	ctxCancel, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err = engineLong.Spawn(ctxCancel, "cancel task", "", 8)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestEngine_DistillationAndPersistence(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	prov := &testProvider{
		replyText: "Task complete.\n\n## Key Learnings\n1. Redis v7 requires ACL rules.\n2. Connection pools must be sized to 20.\n",
	}
	cfg := config.SubagentsConfig{
		Enabled:         true,
		MaxConcurrent:   3,
		MaxDepth:        1,
		DefaultTimeout:  5 * time.Second,
		MaxTurns:        8,
		LearningEnabled: true,
		MaxObservations: 3,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	task := "Investigate Redis setup"
	res, err := engine.Spawn(ctx, task, "Context info", 8)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if !strings.Contains(res, "Task complete.") {
		t.Errorf("Spawn result = %q, want containing 'Task complete.'", res)
	}

	parent.mu.Lock()
	defer parent.mu.Unlock()

	if len(parent.observations) != 2 {
		t.Fatalf("parent.observations len = %d, want 2", len(parent.observations))
	}
	if parent.observations[0].TopicKey != "subagent/investigate-redis-setup/1" {
		t.Errorf("obs[0].TopicKey = %q, want 'subagent/investigate-redis-setup/1'", parent.observations[0].TopicKey)
	}
	if parent.observations[0].Type != "discovery" {
		t.Errorf("obs[0].Type = %q, want 'discovery'", parent.observations[0].Type)
	}
	if parent.observations[0].Content != "Redis v7 requires ACL rules." {
		t.Errorf("obs[0].Content = %q, want 'Redis v7 requires ACL rules.'", parent.observations[0].Content)
	}
	if parent.observations[0].Importance != 3 {
		t.Errorf("obs[0].Importance = %d, want 3", parent.observations[0].Importance)
	}

	var foundAudit bool
	for _, entry := range parent.auditEntries {
		if entry.Category == "learning" && entry.Backend == "subagent" {
			foundAudit = true
			if entry.Decision != "allow" {
				t.Errorf("audit.Decision = %q, want 'allow'", entry.Decision)
			}
			if !strings.Contains(entry.Subject, "distilled 2 observations") {
				t.Errorf("audit.Subject = %q, want containing 'distilled 2 observations'", entry.Subject)
			}
			if !strings.Contains(entry.Subject, task) {
				t.Errorf("audit.Subject = %q, want containing task name %q", entry.Subject, task)
			}
		}
	}
	if !foundAudit {
		t.Errorf("expected audit entry with category 'learning' and backend 'subagent', got: %+v", parent.auditEntries)
	}
}

func TestEngine_DistillationSkippedWhenLearningDisabled(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	prov := &testProvider{
		replyText: "Output\n## Key Learnings\n1. Skip me.\n",
	}
	cfg := config.SubagentsConfig{
		Enabled:         true,
		MaxConcurrent:   3,
		MaxDepth:        1,
		DefaultTimeout:  5 * time.Second,
		MaxTurns:        8,
		LearningEnabled: false,
		MaxObservations: 3,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	_, err := engine.Spawn(ctx, "Do work", "", 8)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	parent.mu.Lock()
	defer parent.mu.Unlock()

	if len(parent.observations) != 0 {
		t.Errorf("expected 0 observations when LearningEnabled is false, got %d", len(parent.observations))
	}
	for _, entry := range parent.auditEntries {
		if entry.Category == "learning" {
			t.Errorf("expected no learning audit entry when LearningEnabled is false, got %+v", entry)
		}
	}
}

func TestEngine_DistillationGracefulDegradationOnSaveError(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	parent.saveObsErr = errors.New("sqlite database locked")

	prov := &testProvider{
		replyText: "Finished.\n## Key Learnings\n1. Handled gracefully.\n",
	}
	cfg := config.SubagentsConfig{
		Enabled:         true,
		MaxConcurrent:   3,
		MaxDepth:        1,
		DefaultTimeout:  5 * time.Second,
		MaxTurns:        8,
		LearningEnabled: true,
		MaxObservations: 3,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	res, err := engine.Spawn(ctx, "Degradation test", "", 8)
	if err != nil {
		t.Fatalf("Spawn should succeed even if SaveObservations fails, got: %v", err)
	}
	if !strings.Contains(res, "Finished.") {
		t.Errorf("Spawn result = %q, want containing 'Finished.'", res)
	}
}

func TestEngine_DistillationGracefulDegradationOnAuditError(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	parent := newFakeParentRepo()
	parent.appendAuditErr = errors.New("audit log write error")

	prov := &testProvider{
		replyText: "Finished.\n## Key Learnings\n1. Audit fail gracefully.\n",
	}
	cfg := config.SubagentsConfig{
		Enabled:         true,
		MaxConcurrent:   3,
		MaxDepth:        1,
		DefaultTimeout:  5 * time.Second,
		MaxTurns:        8,
		LearningEnabled: true,
		MaxObservations: 3,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	res, err := engine.Spawn(ctx, "Audit degradation test", "", 8)
	if err != nil {
		t.Fatalf("Spawn should succeed even if AppendAudit fails, got: %v", err)
	}
	if !strings.Contains(res, "Finished.") {
		t.Errorf("Spawn result = %q, want containing 'Finished.'", res)
	}
}
