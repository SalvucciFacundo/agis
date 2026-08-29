package plugins_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/plugins"
)

func createTestPlugin(t *testing.T, baseDir, pluginName, manifestJSON string, files map[string]string) string {
	t.Helper()
	pluginDir := filepath.Join(baseDir, pluginName)
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", pluginDir, err)
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", manifestPath, err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(pluginDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatalf("MkdirAll for %s: %v", fullPath, err)
		}
		perm := os.FileMode(0o600)
		if filepath.Ext(fullPath) == ".sh" || filepath.Base(filepath.Dir(fullPath)) == "bin" {
			perm = 0o700
		}
		if err := os.WriteFile(fullPath, []byte(content), perm); err != nil {
			t.Fatalf("WriteFile %s: %v", fullPath, err)
		}
	}

	return pluginDir
}

func TestManager_LoadAndList(t *testing.T) {
	pluginsDir := t.TempDir()

	// Plugin 1: weather
	createTestPlugin(t, pluginsDir, "weather", `{
		"name": "weather",
		"version": "1.0.0",
		"description": "Weather forecasting plugin"
	}`, nil)

	// Plugin 2: github
	createTestPlugin(t, pluginsDir, "github", `{
		"name": "github",
		"version": "2.0.0",
		"description": "GitHub integration plugin"
	}`, nil)

	// Invalid dir: no plugin.json
	os.MkdirAll(filepath.Join(pluginsDir, "empty-dir"), 0o700)

	// Invalid dir: invalid plugin.json
	createTestPlugin(t, pluginsDir, "invalid-json", `{bad json}`, nil)

	mgr := plugins.NewManager(plugins.WithStateDir(pluginsDir))
	if err := mgr.Load(pluginsDir); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d plugins, want 2", len(list))
	}

	info, err := mgr.Get("weather")
	if err != nil {
		t.Fatalf("Get(weather) error: %v", err)
	}
	if info.Manifest.Name != "weather" || info.Enabled {
		t.Errorf("unexpected weather info: %+v", info)
	}

	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent plugin, got nil")
	}
}

func TestManager_EnableDisableAndPersistence(t *testing.T) {
	pluginsDir := t.TempDir()

	createTestPlugin(t, pluginsDir, "weather", `{
		"name": "weather",
		"version": "1.0.0"
	}`, nil)

	createTestPlugin(t, pluginsDir, "alerts", `{
		"name": "alerts",
		"version": "1.0.0"
	}`, nil)

	mgr := plugins.NewManager(plugins.WithStateDir(pluginsDir))
	if err := mgr.Load(pluginsDir); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Initially disabled
	info, _ := mgr.Get("weather")
	if info.Enabled {
		t.Errorf("weather should be disabled initially")
	}

	// Enable weather
	if err := mgr.Enable("weather"); err != nil {
		t.Fatalf("Enable(weather) error: %v", err)
	}

	info, _ = mgr.Get("weather")
	if !info.Enabled {
		t.Errorf("weather should be enabled after Enable()")
	}

	// Verify persistence with a new Manager instance loading from disk
	mgr2 := plugins.NewManager(plugins.WithStateDir(pluginsDir))
	if err := mgr2.Load(pluginsDir); err != nil {
		t.Fatalf("Load() on mgr2 error: %v", err)
	}

	info2, _ := mgr2.Get("weather")
	if !info2.Enabled {
		t.Errorf("weather should persist enabled state across managers")
	}
	infoAlerts, _ := mgr2.Get("alerts")
	if infoAlerts.Enabled {
		t.Errorf("alerts should remain disabled")
	}

	// Disable weather
	if err := mgr2.Disable("weather"); err != nil {
		t.Fatalf("Disable(weather) error: %v", err)
	}
	info2, _ = mgr2.Get("weather")
	if info2.Enabled {
		t.Errorf("weather should be disabled after Disable()")
	}

	// Nonexistent enable/disable errors
	if err := mgr2.Enable("unknown"); err == nil {
		t.Error("expected error enabling unknown plugin")
	}
	if err := mgr2.Disable("unknown"); err == nil {
		t.Error("expected error disabling unknown plugin")
	}
}

func TestManager_ToolRunners(t *testing.T) {
	pluginsDir := t.TempDir()

	scriptContent := "#!/bin/sh\necho \"plugin executed: $1\"\n"
	createTestPlugin(t, pluginsDir, "echoer", `{
		"name": "echoer",
		"version": "1.0.0",
		"entrypoint": "bin/run.sh",
		"tools": [
			{"name": "echo_tool", "description": "Echoes input"}
		]
	}`, map[string]string{
		"bin/run.sh": scriptContent,
	})

	mgr := plugins.NewManager(plugins.WithStateDir(pluginsDir))
	if err := mgr.Load(pluginsDir); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// When disabled, no runners
	if len(mgr.Runners()) != 0 {
		t.Errorf("expected 0 runners for disabled plugin, got %d", len(mgr.Runners()))
	}

	// Enable echoer
	if err := mgr.Enable("echoer"); err != nil {
		t.Fatalf("Enable(echoer) error: %v", err)
	}

	runners := mgr.Runners()
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}

	runner := runners[0]
	if runner.Backend() != "plugin-echoer" {
		t.Errorf("Backend() = %q, want plugin-echoer", runner.Backend())
	}

	ctx := context.Background()
	out, err := runner.Run(ctx, "hello")
	if err != nil {
		t.Fatalf("Run(hello) error: %v", err)
	}
	if !contains(out, "plugin executed: hello") {
		t.Errorf("Run output = %q, want containing 'plugin executed: hello'", out)
	}
}

func TestManager_Skills(t *testing.T) {
	pluginsDir := t.TempDir()

	skillContent := `---
name: deploy-helper
description: Helps with deploying apps
trigger: deploy application
---
Use the deploy tool carefully.
`

	createTestPlugin(t, pluginsDir, "deployer", `{
		"name": "deployer",
		"version": "1.0.0",
		"skills": ["deploy-skill.md"]
	}`, map[string]string{
		"deploy-skill.md": skillContent,
	})

	mgr := plugins.NewManager(plugins.WithStateDir(pluginsDir))
	if err := mgr.Load(pluginsDir); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// When disabled, no skills
	if len(mgr.Skills()) != 0 {
		t.Errorf("expected 0 skills for disabled plugin, got %d", len(mgr.Skills()))
	}

	// Enable deployer
	if err := mgr.Enable("deployer"); err != nil {
		t.Fatalf("Enable(deployer) error: %v", err)
	}

	sk := mgr.Skills()
	if len(sk) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(sk))
	}
	if sk[0].Name != "deploy-helper" {
		t.Errorf("Skill.Name = %q, want deploy-helper", sk[0].Name)
	}
}

