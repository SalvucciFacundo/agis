package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// SubagentSpawner defines the contract for spawning isolated child subagents.
type SubagentSpawner interface {
	Spawn(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error)
}

// SubagentRunner implements core.ToolRunner for native subagent delegation.
type SubagentRunner struct {
	spawner SubagentSpawner
}

// NewSubagentRunner constructs a new SubagentRunner with the given spawner.
func NewSubagentRunner(spawner SubagentSpawner) *SubagentRunner {
	return &SubagentRunner{
		spawner: spawner,
	}
}

// FromSubagentsEngine creates a core.ToolRunner backed by a SubagentSpawner.
func FromSubagentsEngine(engine SubagentSpawner) core.ToolRunner {
	return NewSubagentRunner(engine)
}

// Backend implements core.ToolRunner.
func (r *SubagentRunner) Backend() string {
	return "subagent"
}

// Name implements core.ToolRunner.
func (r *SubagentRunner) Name() string {
	return "delegate_task"
}

// Description implements core.ToolRunner.
func (r *SubagentRunner) Description() string {
	return "Delegate a focused task to an isolated, bounded subagent instance. The subagent runs its own execution loop and returns a synthesized summary of the result."
}

type delegateTaskArgs struct {
	Task     string `json:"task"`
	Context  string `json:"context"`
	MaxTurns int    `json:"max_turns"`
}

func parseDelegateTaskArgs(input string) (delegateTaskArgs, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return delegateTaskArgs{}, errors.New("task parameter is required and cannot be empty")
	}

	var args delegateTaskArgs
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return delegateTaskArgs{}, fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		args.Task = trimmed
	}

	args.Task = strings.TrimSpace(args.Task)
	if args.Task == "" {
		return delegateTaskArgs{}, errors.New("task parameter is required and cannot be empty")
	}

	if args.MaxTurns <= 0 {
		args.MaxTurns = 8
	} else if args.MaxTurns > 15 {
		args.MaxTurns = 15
	}

	return args, nil
}

// Run executes a delegated task via the underlying SubagentSpawner.
func (r *SubagentRunner) Run(ctx context.Context, command string) (string, error) {
	if r.spawner == nil {
		return "", errors.New("subagent spawner not available")
	}

	args, err := parseDelegateTaskArgs(command)
	if err != nil {
		return "", err
	}

	return r.spawner.Spawn(ctx, args.Task, args.Context, args.MaxTurns)
}
