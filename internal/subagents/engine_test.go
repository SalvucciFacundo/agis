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
