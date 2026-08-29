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

func TestGatewayCLI_HelpAndInvalidFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunGatewayCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunGatewayCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") && !strings.Contains(stdout.String(), "gateway") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestGatewayCLI_DisabledGateway(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("gateway:\n  enabled: false\n"), 0o600)

	var stdout, stderr bytes.Buffer
	code := RunGatewayCLI([]string{"run", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("RunGatewayCLI(disabled gateway) = 0, want non-zero error code")
	}
	if !strings.Contains(stderr.String(), "disabled") && !strings.Contains(stderr.String(), "enabled") {
		t.Errorf("stderr = %q, want mention of disabled gateway", stderr.String())
	}
}

func TestGatewayCLI_RunWithContextCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
gateway:
  enabled: true
  telegram:
    enabled: true
    token: "mock-token"
    allowlist: ["123"]
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runGatewayWithContext(ctx, []string{"run", "--config", configPath}, &stdout, &stderr)
	}()

	// Wait briefly for daemon to initialize, then cancel context to simulate SIGINT
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("runGatewayWithContext exit code = %d, want 0 on graceful shutdown", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("gateway daemon did not shut down within timeout")
	}
}
