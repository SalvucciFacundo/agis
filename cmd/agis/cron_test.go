package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCronCLI_HelpAndInvalidFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCronCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunCronCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") && !strings.Contains(stdout.String(), "cron") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestCronCLI_List_EmptyAndPopulated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	// 1. Empty config
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("cron:\n  enabled: false\n"), 0o600)

	var stdout, stderr bytes.Buffer
	code := RunCronCLI([]string{"list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCronCLI(list empty) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "No cron jobs configured") {
		t.Errorf("stdout = %q, want mention of no jobs configured", stdout.String())
	}

	// 2. Populated config
	stdout.Reset()
	stderr.Reset()
	_ = os.WriteFile(configPath, []byte(`
cron:
  enabled: true
  jobs:
    - name: "daily-health"
      schedule: "@every 1h"
      prompt: "Check system health"
      session_id: "health-session"
      target:
        adapter: "telegram"
        recipient: "123456"
    - name: "nightly-backup"
      schedule: "0 2 * * *"
      prompt: "Backup database"
`), 0o600)

	code = RunCronCLI([]string{"list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunCronCLI(list populated) = %d, want 0", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "daily-health") || !strings.Contains(out, "@every 1h") || !strings.Contains(out, "telegram") {
		t.Errorf("stdout = %q, want details for daily-health job", out)
	}
	if !strings.Contains(out, "nightly-backup") || !strings.Contains(out, "0 2 * * *") {
		t.Errorf("stdout = %q, want details for nightly-backup job", out)
	}
}

func TestCronCLI_DisabledCron(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("cron:\n  enabled: false\n"), 0o600)

	var stdout, stderr bytes.Buffer
	code := RunCronCLI([]string{"run", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("RunCronCLI(disabled cron run) = 0, want non-zero error code")
	}
	if !strings.Contains(stderr.String(), "disabled") {
		t.Errorf("stderr = %q, want mention of disabled cron", stderr.String())
	}
}

func TestCronCLI_RunWithContextCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
cron:
  enabled: true
  jobs:
    - name: "test-heartbeat"
      schedule: "@every 10s"
      prompt: "Heartbeat check"
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr safeBuffer
	done := make(chan int, 1)
	go func() {
		done <- runCronWithContext(ctx, []string{"run", "--config", configPath}, &stdout, &stderr)
	}()

	// Wait until daemon initializes, then cancel context to simulate SIGINT
	for i := 0; i < 100; i++ {
		if strings.Contains(stdout.String(), "running") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("runCronWithContext exit code = %d, want 0 on graceful shutdown", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cron daemon did not shut down within timeout")
	}
}
