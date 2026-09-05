package subagents_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/subagents"
	"go.uber.org/goleak"
)

type dummyRunner struct {
	name    string
	backend string
}

func (r *dummyRunner) Name() string                                { return r.name }
func (r *dummyRunner) Description() string                         { return "dummy runner" }
func (r *dummyRunner) Backend() string                             { return r.backend }
func (r *dummyRunner) Run(context.Context, string) (string, error) { return "ok", nil }

func TestEngine_DepthTrackingAndExceeded(t *testing.T) {
	defer goleak.VerifyNone(t)

	parent := newFakeParentRepo()
	prov := &testProvider{replyText: "deep reply"}

	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2,
		MaxDepth:       1, // Max depth 1
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       8,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, nil)

	// Context at depth 0 should succeed
	ctx0 := subagents.ContextWithDepth(context.Background(), 0)
	res, err := engine.Spawn(ctx0, "task depth 0", "", 8)
	if err != nil {
		t.Fatalf("Spawn depth 0 failed: %v", err)
	}
	if res != "deep reply" {
		t.Errorf("expected 'deep reply', got %q", res)
	}

	// Context already at depth 1 should fail with recursion limit exceeded
	ctx1 := subagents.ContextWithDepth(context.Background(), 1)
	_, err = engine.Spawn(ctx1, "task depth 1", "", 8)
	if err == nil || !strings.Contains(err.Error(), "recursion depth limit (1) exceeded") {
		t.Fatalf("expected recursion depth limit error, got: %v", err)
	}
}

func TestEngine_ToolInheritanceFilteringAtMaxDepth(t *testing.T) {
	defer goleak.VerifyNone(t)

	parent := newFakeParentRepo()

	var receivedTools []core.ToolDef
	prov := &testProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			receivedTools = req.Tools
			ch := make(chan core.StreamEvent, 1)
			ch <- core.StreamEvent{Text: "tool test reply"}
			close(ch)
			return ch, nil
		},
	}

	runners := []core.ToolRunner{
		&dummyRunner{name: "local_tool", backend: "local"},
		&dummyRunner{name: "delegate_task", backend: "subagent"},
	}

	// MaxDepth 1: When root (depth 0) spawns child (depth 1), nextDepth == MaxDepth,
	// so delegate_task MUST be filtered out of child's tools.
	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2,
		MaxDepth:       1,
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       8,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, runners)

	_, err := engine.Spawn(context.Background(), "filter tools test", "", 8)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	for _, tool := range receivedTools {
		if tool.Name == "delegate_task" {
			t.Errorf("delegate_task should have been filtered out for depth 1 child, but was present in %+v", receivedTools)
		}
	}
}

func TestEngine_TurnLimitExhaustionWarning(t *testing.T) {
	defer goleak.VerifyNone(t)

	parent := newFakeParentRepo()

	round := 0
	prov := &testProvider{
		streamFunc: func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
			ch := make(chan core.StreamEvent, 2)
			round++
			if round <= 2 {
				// emit tool call
				ch <- core.StreamEvent{ToolCall: &core.ToolCall{ID: "tc-1", Name: "local_tool", Arguments: `{"command":"status"}`}}
			} else {
				ch <- core.StreamEvent{Text: "partial conclusion"}
			}
			close(ch)
			return ch, nil
		},
	}

	runners := []core.ToolRunner{
		&dummyRunner{name: "local_tool", backend: "local"},
	}

	// Set MaxTurns = 2 to force exhaustion
	cfg := config.SubagentsConfig{
		Enabled:        true,
		MaxConcurrent:  2,
		MaxDepth:       2,
		DefaultTimeout: 5 * time.Second,
		MaxTurns:       2,
	}

	engine := subagents.NewEngine(cfg, parent, prov, &testGuard{decision: core.DecisionAllow}, nil, runners)

	res, err := engine.Spawn(context.Background(), "exhaustion test", "", 2)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if !strings.Contains(res, "[subagent reached maximum turn limit (2)]") {
		t.Errorf("expected turn limit warning in result, got: %q", res)
	}
}
