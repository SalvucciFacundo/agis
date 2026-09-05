package subagents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

type contextKey string

const depthKey contextKey = "subagentDepth"

// DepthFromContext returns the current subagent recursion depth.
func DepthFromContext(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	if val, ok := ctx.Value(depthKey).(int); ok {
		return val
	}
	return 0
}

// ContextWithDepth returns a new context carrying the given subagent recursion depth.
func ContextWithDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, depthKey, depth)
}

// Engine manages the execution lifecycle of child subagents.
type Engine struct {
	cfg      config.SubagentsConfig
	parent   core.Repository
	provider core.Provider
	guard    core.PolicyGuard
	approver core.Approver
	runners  []core.ToolRunner
	sem      chan struct{}
}

// NewEngine creates a new subagent execution Engine.
func NewEngine(
	cfg config.SubagentsConfig,
	parent core.Repository,
	provider core.Provider,
	guard core.PolicyGuard,
	approver core.Approver,
	runners []core.ToolRunner,
) *Engine {
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	} else if maxConcurrent > 10 {
		maxConcurrent = 10
	}

	return &Engine{
		cfg:      cfg,
		parent:   parent,
		provider: provider,
		guard:    guard,
		approver: approver,
		runners:  runners,
		sem:      make(chan struct{}, maxConcurrent),
	}
}

// Spawn executes a delegated task in an isolated ephemeral child brain.
func (e *Engine) Spawn(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error) {
	if !e.cfg.Enabled {
		return "", errors.New("subagent delegation is disabled by configuration")
	}

	trimmedTask := strings.TrimSpace(task)
	if trimmedTask == "" {
		return "", errors.New("task parameter is required and cannot be empty")
	}

	currentDepth := DepthFromContext(ctx)
	maxDepth := e.cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	} else if maxDepth > 2 {
		maxDepth = 2
	}

	if currentDepth >= maxDepth {
		return "", fmt.Errorf("recursion depth limit (%d) exceeded", maxDepth)
	}

	effectiveMaxTurns := maxTurns
	if effectiveMaxTurns <= 0 {
		effectiveMaxTurns = e.cfg.MaxTurns
		if effectiveMaxTurns <= 0 {
			effectiveMaxTurns = 8
		}
	}
	if effectiveMaxTurns > 15 {
		effectiveMaxTurns = 15
	}

	timeout := e.cfg.DefaultTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	} else if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	childCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	nextDepth := currentDepth + 1
	childCtx = ContextWithDepth(childCtx, nextDepth)

	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-childCtx.Done():
		return "", childCtx.Err()
	}

	var childRunners []core.ToolRunner
	for _, r := range e.runners {
		if nextDepth >= maxDepth {
			if r.Backend() == "subagent" || r.Name() == "delegate_task" {
				continue
			}
		}
		childRunners = append(childRunners, r)
	}

	ephemeralRepo := NewEphemeralRepository(e.parent)

	childIdentity := "You are a focused subagent tasked with executing a specific task. Analyze the input, use available tools if necessary, and return a clear, concise, and synthesized response."

	var brainOpts []core.Option
	brainOpts = append(brainOpts, core.WithIdentity(childIdentity))
	if len(childRunners) > 0 && e.guard != nil {
		brainOpts = append(brainOpts, core.WithTools(childRunners, e.guard, e.approver))
	}
	brainOpts = append(brainOpts, core.WithMaxTurns(effectiveMaxTurns))

	childBrain := core.NewBrain(ephemeralRepo, e.provider, brainOpts...)

	userPrompt := trimmedTask
	if ctxInfo := strings.TrimSpace(contextInfo); ctxInfo != "" {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nTask:\n%s", ctxInfo, trimmedTask)
	}

	if err := childBrain.Step(childCtx, userPrompt); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", fmt.Errorf("subagent execution failed: provider error: %w", err)
	}

	conv, err := ephemeralRepo.LatestConversation(childCtx)
	if err != nil {
		return "", fmt.Errorf("retrieving subagent response: %w", err)
	}

	msgs, err := ephemeralRepo.Messages(childCtx, conv.ID, 10)
	if err != nil {
		return "", fmt.Errorf("retrieving subagent messages: %w", err)
	}

	var reply string
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == core.RoleAssistant {
			reply = msgs[i].Content
			break
		}
	}

	if childBrain.TurnLimitReached() {
		reply += fmt.Sprintf("\n[subagent reached maximum turn limit (%d)]", effectiveMaxTurns)
	}

	return reply, nil
}
