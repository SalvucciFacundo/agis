package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// LoadToolRunner implements core.ToolRunner for lazy tool schema loading.
type LoadToolRunner struct {
	inventory    []core.ToolRunner
	getInventory func() []core.ToolRunner
}

// NewLoadToolRunner creates a LoadToolRunner backed by a static inventory slice.
func NewLoadToolRunner(inventory []core.ToolRunner) *LoadToolRunner {
	return &LoadToolRunner{
		inventory: inventory,
	}
}

// NewLoadToolRunnerWithGetter creates a LoadToolRunner backed by a dynamic inventory getter.
func NewLoadToolRunnerWithGetter(getter func() []core.ToolRunner) *LoadToolRunner {
	return &LoadToolRunner{
		getInventory: getter,
	}
}

// Backend implements core.ToolRunner.
func (l *LoadToolRunner) Backend() string {
	return "internal"
}

// Name implements core.ToolRunner.
func (l *LoadToolRunner) Name() string {
	return "load_tool"
}

// Description implements core.ToolRunner.
func (l *LoadToolRunner) Description() string {
	return "Load the full schema and enable a specific tool for the current conversation turn."
}

// Run looks up the requested tool and returns a confirmation JSON payload.
func (l *LoadToolRunner) Run(_ context.Context, input string) (string, error) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("parsing load_tool arguments: %w", err)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", errors.New("tool name is required")
	}

	runners := l.inventory
	if l.getInventory != nil {
		runners = l.getInventory()
	}

	var found core.ToolRunner
	for _, r := range runners {
		rName := r.Name()
		if rName == "" {
			rName = "shell-" + r.Backend()
		}
		if rName == name {
			found = r
			break
		}
	}

	if found == nil {
		return "", fmt.Errorf("tool %q not found", name)
	}

	resp := struct {
		Status      string `json:"status"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		Status:      "loaded",
		Name:        name,
		Description: found.Description(),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("marshaling load_tool response: %w", err)
	}

	return string(data), nil
}
