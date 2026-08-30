package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLocal_RunCapturesOutput(t *testing.T) {
	l := NewLocal(5 * time.Second)
	out, err := l.Run(context.Background(), "echo hello-tools")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "hello-tools") {
		t.Errorf("out = %q, want captured stdout", out)
	}
}

func TestLocal_BackendName(t *testing.T) {
	l := NewLocal(0)
	if got := l.Backend(); got != "local" {
		t.Errorf("Backend() = %q, want local", got)
	}
	if got := l.Name(); got != "shell-local" {
		t.Errorf("Name() = %q, want shell-local", got)
	}
	if got := l.Description(); !strings.Contains(got, "local") {
		t.Errorf("Description() = %q, want description containing local", got)
	}
}

func TestLocal_FailureSurfacesWithOutput(t *testing.T) {
	l := NewLocal(5 * time.Second)
	out, err := l.Run(context.Background(), "echo before && false")
	if err == nil {
		t.Fatal("Run() error = nil for failing command")
	}
	if !strings.Contains(out, "before") {
		t.Errorf("out = %q, want partial output preserved", out)
	}
}

func TestLocal_TimeoutBoundsRunaway(t *testing.T) {
	l := NewLocal(150 * time.Millisecond)
	start := time.Now()
	if _, err := l.Run(context.Background(), "sleep 5"); err == nil {
		t.Fatal("runaway command not terminated")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("elapsed = %v, want ~150ms timeout", elapsed)
	}
}
