package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createMCPTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return cfgPath
}

func TestRunMCPCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage: agis mcp") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestRunMCPCLI_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"unknown-cmd"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("stderr = %q, want unknown subcommand error", stderr.String())
	}
}

func TestRunMCPCLI_List_NoServers(t *testing.T) {
	cfgPath := createMCPTestConfig(t, `
mcp:
  enabled: true
  servers: {}
`)
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"list", "-config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No MCP servers configured") {
		t.Errorf("stdout = %q, want 'No MCP servers configured'", stdout.String())
	}
}

func TestRunMCPCLI_List_DisabledServer(t *testing.T) {
	cfgPath := createMCPTestConfig(t, `
mcp:
  enabled: true
  servers:
    local-fs:
      command: "nonexistent-cmd"
      disabled: true
`)
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"list", "-config", cfgPath}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "local-fs") || !strings.Contains(out, "disabled") {
		t.Errorf("stdout = %q, want listing local-fs with disabled status", out)
	}
}

func TestRunMCPCLI_Test_MissingArgs(t *testing.T) {
	cfgPath := createMCPTestConfig(t, `
mcp:
  enabled: true
  servers: {}
`)
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"test", "-config", cfgPath}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "server and tool arguments are required") {
		t.Errorf("stderr = %q, want required arguments error", stderr.String())
	}
}

func TestRunMCPCLI_Test_UnknownServer(t *testing.T) {
	cfgPath := createMCPTestConfig(t, `
mcp:
  enabled: true
  servers:
    my-server:
      command: "echo"
`)
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"test", "nonexistent-server", "echo", "-config", cfgPath}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "server \"nonexistent-server\" not found in configuration") {
		t.Errorf("stderr = %q, want not found in configuration error", stderr.String())
	}
}

func TestRunMCPCLI_Test_DisabledServer(t *testing.T) {
	cfgPath := createMCPTestConfig(t, `
mcp:
  enabled: true
  servers:
    dis-server:
      command: "echo"
      disabled: true
`)
	var stdout, stderr bytes.Buffer
	code := RunMCPCLI([]string{"test", "dis-server", "echo", "-config", cfgPath}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "server \"dis-server\" is disabled") {
		t.Errorf("stderr = %q, want disabled error", stderr.String())
	}
}
