package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeSpawner struct {
	spawnFunc func(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error)
	calls     []spawnCall
}

type spawnCall struct {
	task        string
	contextInfo string
	maxTurns    int
}

func (f *fakeSpawner) Spawn(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error) {
	f.calls = append(f.calls, spawnCall{
		task:        task,
		contextInfo: contextInfo,
		maxTurns:    maxTurns,
	})
	if f.spawnFunc != nil {
		return f.spawnFunc(ctx, task, contextInfo, maxTurns)
	}
	return "synthesis output", nil
}

func TestSubagentRunner_Metadata(t *testing.T) {
	spawner := &fakeSpawner{}
	runner := NewSubagentRunner(spawner)

	if runner.Name() != "delegate_task" {
		t.Errorf("runner.Name() = %q, want %q", runner.Name(), "delegate_task")
	}
	if runner.Backend() != "subagent" {
		t.Errorf("runner.Backend() = %q, want %q", runner.Backend(), "subagent")
	}
	if runner.Description() == "" {
		t.Errorf("runner.Description() is empty")
	}
}

func TestSubagentRunner_Run_JSONArguments(t *testing.T) {
	spawner := &fakeSpawner{
		spawnFunc: func(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error) {
			if task != "analyze error logs" {
				t.Errorf("unexpected task: %q", task)
			}
			if contextInfo != "log dump attached" {
				t.Errorf("unexpected contextInfo: %q", contextInfo)
			}
			if maxTurns != 6 {
				t.Errorf("unexpected maxTurns: %d", maxTurns)
			}
			return "found 2 errors", nil
		},
	}

	runner := NewSubagentRunner(spawner)
	input := `{"task": "analyze error logs", "context": "log dump attached", "max_turns": 6}`
	out, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "found 2 errors" {
		t.Errorf("Run() output = %q, want %q", out, "found 2 errors")
	}
	if len(spawner.calls) != 1 {
		t.Fatalf("spawner called %d times, want 1", len(spawner.calls))
	}
}

func TestSubagentRunner_Run_RawStringArgument(t *testing.T) {
	spawner := &fakeSpawner{}
	runner := NewSubagentRunner(spawner)

	out, err := runner.Run(context.Background(), "run quick calculation")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "synthesis output" {
		t.Errorf("Run() output = %q, want %q", out, "synthesis output")
	}
	if len(spawner.calls) != 1 {
		t.Fatalf("spawner called %d times, want 1", len(spawner.calls))
	}
	if spawner.calls[0].task != "run quick calculation" {
		t.Errorf("spawner task = %q, want %q", spawner.calls[0].task, "run quick calculation")
	}
	if spawner.calls[0].maxTurns != 8 {
		t.Errorf("spawner maxTurns = %d, want default 8", spawner.calls[0].maxTurns)
	}
}

func TestSubagentRunner_Run_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: "task parameter is required and cannot be empty",
		},
		{
			name:    "whitespace string",
			input:   "    \n\t  ",
			wantErr: "task parameter is required and cannot be empty",
		},
		{
			name:    "empty task in JSON",
			input:   `{"task": ""}`,
			wantErr: "task parameter is required and cannot be empty",
		},
		{
			name:    "whitespace task in JSON",
			input:   `{"task": "   \t  "}`,
			wantErr: "task parameter is required and cannot be empty",
		},
		{
			name:    "invalid json starting with brace",
			input:   `{"task": broken json`,
			wantErr: "invalid json arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spawner := &fakeSpawner{}
			runner := NewSubagentRunner(spawner)
			_, err := runner.Run(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Run() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
			if len(spawner.calls) != 0 {
				t.Errorf("spawner called %d times on validation error, want 0", len(spawner.calls))
			}
		})
	}
}

func TestSubagentRunner_Run_MaxTurnsClamping(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantMaxTurns int
	}{
		{
			name:         "default when zero",
			input:        `{"task": "test", "max_turns": 0}`,
			wantMaxTurns: 8,
		},
		{
			name:         "default when negative",
			input:        `{"task": "test", "max_turns": -5}`,
			wantMaxTurns: 8,
		},
		{
			name:         "clamped when exceeding 15",
			input:        `{"task": "test", "max_turns": 100}`,
			wantMaxTurns: 15,
		},
		{
			name:         "valid within range",
			input:        `{"task": "test", "max_turns": 12}`,
			wantMaxTurns: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spawner := &fakeSpawner{}
			runner := NewSubagentRunner(spawner)
			_, err := runner.Run(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if len(spawner.calls) != 1 {
				t.Fatalf("spawner called %d times, want 1", len(spawner.calls))
			}
			if spawner.calls[0].maxTurns != tt.wantMaxTurns {
				t.Errorf("spawner maxTurns = %d, want %d", spawner.calls[0].maxTurns, tt.wantMaxTurns)
			}
		})
	}
}

func TestSubagentRunner_Run_SpawnerErrors(t *testing.T) {
	t.Run("nil spawner", func(t *testing.T) {
		runner := NewSubagentRunner(nil)
		_, err := runner.Run(context.Background(), `{"task": "test"}`)
		if err == nil || !strings.Contains(err.Error(), "subagent spawner not available") {
			t.Errorf("expected error about nil spawner, got: %v", err)
		}
	})

	t.Run("spawner failure", func(t *testing.T) {
		spawner := &fakeSpawner{
			spawnFunc: func(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error) {
				return "", errors.New("rate limit exceeded")
			},
		}
		runner := NewSubagentRunner(spawner)
		_, err := runner.Run(context.Background(), `{"task": "test"}`)
		if err == nil || !strings.Contains(err.Error(), "rate limit exceeded") {
			t.Errorf("expected spawner error propagated, got: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		spawner := &fakeSpawner{
			spawnFunc: func(ctx context.Context, task string, contextInfo string, maxTurns int) (string, error) {
				return "", ctx.Err()
			},
		}
		runner := NewSubagentRunner(spawner)
		_, err := runner.Run(ctx, `{"task": "test"}`)
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	})
}

func TestFromSubagentsEngine(t *testing.T) {
	spawner := &fakeSpawner{}
	runner := FromSubagentsEngine(spawner)
	if runner == nil {
		t.Fatalf("FromSubagentsEngine returned nil")
	}
	if runner.Name() != "delegate_task" {
		t.Errorf("runner.Name() = %q, want %q", runner.Name(), "delegate_task")
	}
	if runner.Backend() != "subagent" {
		t.Errorf("runner.Backend() = %q, want %q", runner.Backend(), "subagent")
	}
}
