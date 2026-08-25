// Package tools implements the M4 execution backends behind the core.ToolRunner
// port: local shell, Docker containers, and SSH remotes. Every runner is
// policy-gated upstream by the Policy Guard; runners execute whatever they are
// told and never make permission decisions themselves.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// defaultLocalTimeout bounds one local command.
const defaultLocalTimeout = 60 * time.Second

// Local executes commands on the host through sh -c.
type Local struct {
	timeout time.Duration
}

// NewLocal returns a local runner with the given per-command timeout; a
// non-positive timeout falls back to the 60s default.
func NewLocal(timeout time.Duration) *Local {
	if timeout <= 0 {
		timeout = defaultLocalTimeout
	}
	return &Local{timeout: timeout}
}

// Backend implements core.ToolRunner.
func (l *Local) Backend() string { return "local" }

// Run executes command via sh -c and returns combined output.
func (l *Local) Run(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("local command failed: %w", err)
	}
	return string(out), nil
}
