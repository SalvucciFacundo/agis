package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createCLIPlugin(t *testing.T, baseDir, pluginName, manifestJSON string) string {
	t.Helper()
	pluginDir := filepath.Join(baseDir, pluginName)
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", pluginDir, err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", manifestPath, err)
	}
	return pluginDir
}

func TestPluginsCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunPluginsCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunPluginsCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "plugins") {
		t.Errorf("stdout = %q, want usage output", stdout.String())
	}
}

func TestPluginsCLI_ListEnableDisableInspect(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	pluginsDir := filepath.Join(home, "plugins")
	createCLIPlugin(t, pluginsDir, "weather", `{
		"name": "weather",
		"version": "1.0.0",
		"description": "Weather plugin",
		"entrypoint": "bin/weather",
		"tools": [
			{"name": "get_weather", "description": "Get current weather"}
		],
		"skills": ["weather.md"],
		"permissions": ["network"]
	}`)

	createCLIPlugin(t, pluginsDir, "jira", `{
		"name": "jira",
		"version": "2.3.1",
		"description": "Jira issue tracker"
	}`)

	configPath := filepath.Join(home, "config.yaml")
	_ = os.WriteFile(configPath, []byte("plugins:\n  enabled: true\n"), 0o600)

	// 1. List
	var stdout, stderr bytes.Buffer
	code := RunPluginsCLI([]string{"list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPluginsCLI(list) = %d, want 0, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "weather") || !strings.Contains(out, "jira") {
		t.Errorf("stdout = %q, want weather and jira in list", out)
	}

	// 2. Enable weather
	stdout.Reset()
	stderr.Reset()
	code = RunPluginsCLI([]string{"enable", "weather", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPluginsCLI(enable weather) = %d, want 0, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "enabled") {
		t.Errorf("stdout = %q, want enabled message", stdout.String())
	}

	// 3. Inspect weather
	stdout.Reset()
	stderr.Reset()
	code = RunPluginsCLI([]string{"inspect", "weather", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPluginsCLI(inspect weather) = %d, want 0, stderr = %s", code, stderr.String())
	}
	out = stdout.String()
	if !strings.Contains(out, "weather") || !strings.Contains(out, "1.0.0") || !strings.Contains(out, "get_weather") || !strings.Contains(out, "network") {
		t.Errorf("stdout = %q, want inspect details", out)
	}

	// 4. Disable weather
	stdout.Reset()
	stderr.Reset()
	code = RunPluginsCLI([]string{"disable", "weather", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunPluginsCLI(disable weather) = %d, want 0, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "disabled") {
		t.Errorf("stdout = %q, want disabled message", stdout.String())
	}

	// 5. Inspect nonexistent plugin
	stdout.Reset()
	stderr.Reset()
	code = RunPluginsCLI([]string{"inspect", "unknown", "--config", configPath}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("expected non-zero for unknown plugin inspect, got 0")
	}
}
