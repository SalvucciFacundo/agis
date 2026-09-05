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

func TestServeCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunServeCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunServeCLI(--help) = %d, want 0", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "serve") {
		t.Errorf("stdout = %q, want usage output mentioning 'serve'", out)
	}
	if !strings.Contains(out, "-host") || !strings.Contains(out, "-port") || !strings.Contains(out, "-api-key") {
		t.Errorf("stdout = %q, want flags -host, -port, -api-key", out)
	}
}

func TestServeCLI_RunWithContextCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte(`
db:
  path: ":memory:"
server:
  enabled: true
  host: "127.0.0.1"
  port: 0
  api_key: "test-secret-token"
`), 0o600)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stdout, stderr safeBuffer
	doneCh := make(chan int, 1)

	go func() {
		doneCh <- runServeWithContext(ctx, []string{"--config", configPath}, &stdout, &stderr)
	}()

	for i := 0; i < 100; i++ {
		if strings.Contains(stdout.String(), "listening on") || strings.Contains(stderr.String(), "listening on") || strings.Contains(stdout.String(), "API Server") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case code := <-doneCh:
		if code != 0 {
			t.Errorf("runServeWithContext exited with code %d, want 0; stderr = %s", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to shut down after cancel")
	}
}
