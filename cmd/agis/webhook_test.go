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

func TestWebhookCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunWebhookCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunWebhookCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "webhook") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestWebhookCLI_DisabledWebhook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("webhook:\n  enabled: false\n"), 0o600)

	var stdout, stderr bytes.Buffer
	code := RunWebhookCLI([]string{"run", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("RunWebhookCLI(disabled webhook) = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "disabled") {
		t.Errorf("stderr = %q, want mention of disabled webhook", stderr.String())
	}
}

func TestWebhookCLI_RunWithContextCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
webhook:
  enabled: true
  host: "127.0.0.1"
  port: 0
  path: "/webhook"
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr bytes.Buffer
	doneCh := make(chan int, 1)

	go func() {
		doneCh <- runWebhookWithContext(ctx, []string{"run", "--config", configPath}, &stdout, &stderr)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case code := <-doneCh:
		if code != 0 {
			t.Errorf("runWebhookWithContext returned %d, want 0 (stderr = %s)", code, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook daemon did not stop within 3s")
	}
}
