package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"gopkg.in/yaml.v3"
)

func TestSave(t *testing.T) {
	t.Run("saves config to disk with 0600 permissions", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		cfg, err := config.Load("")
		if err != nil {
			t.Fatalf("config.Load() unexpected error: %v", err)
		}

		cfg.LLM.Provider = "anthropic"
		cfg.LLM.Model = "claude-3-5-sonnet"
		cfg.Memory.RecallLimit = 42
		cfg.Memory.CloseTimeout = 50 * time.Second

		if err := config.Save(configPath, cfg); err != nil {
			t.Fatalf("config.Save() unexpected error: %v", err)
		}

		// Check file exists
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("os.Stat(%q) unexpected error: %v", configPath, err)
		}

		// Check strict 0600 permissions
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Errorf("file permissions = %04o, want %04o", perm, 0o600)
		}

		// Check file contents by reading back and unmarshaling
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) unexpected error: %v", configPath, err)
		}

		var reloaded config.Config
		if err := yaml.Unmarshal(data, &reloaded); err != nil {
			t.Fatalf("yaml.Unmarshal() unexpected error: %v", err)
		}

		if reloaded.LLM.Provider != "anthropic" {
			t.Errorf("reloaded.LLM.Provider = %q, want 'anthropic'", reloaded.LLM.Provider)
		}
		if reloaded.LLM.Model != "claude-3-5-sonnet" {
			t.Errorf("reloaded.LLM.Model = %q, want 'claude-3-5-sonnet'", reloaded.LLM.Model)
		}
		if reloaded.Memory.RecallLimit != 42 {
			t.Errorf("reloaded.Memory.RecallLimit = %d, want 42", reloaded.Memory.RecallLimit)
		}
	})

	t.Run("creates non-existent parent directory with 0700", func(t *testing.T) {
		tempDir := t.TempDir()
		nestedDir := filepath.Join(tempDir, "nested", "sub", ".agis")
		configPath := filepath.Join(nestedDir, "config.yaml")

		cfg, err := config.Load("")
		if err != nil {
			t.Fatalf("config.Load() error: %v", err)
		}
		cfg.LLM.Provider = "ollama"

		if err := config.Save(configPath, cfg); err != nil {
			t.Fatalf("config.Save() error: %v", err)
		}

		dirInfo, err := os.Stat(nestedDir)
		if err != nil {
			t.Fatalf("os.Stat(%q) error: %v", nestedDir, err)
		}

		dirPerm := dirInfo.Mode().Perm()
		if dirPerm != 0o700 {
			t.Errorf("directory permissions = %04o, want %04o", dirPerm, 0o700)
		}

		fileInfo, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("os.Stat(%q) error: %v", configPath, err)
		}
		if fileInfo.Mode().Perm() != 0o600 {
			t.Errorf("file permissions = %04o, want %04o", fileInfo.Mode().Perm(), 0o600)
		}
	})

	t.Run("overwrites existing config atomically", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		cfg1, _ := config.Load("")
		cfg1.LLM.Model = "model-v1"
		if err := config.Save(configPath, cfg1); err != nil {
			t.Fatalf("first Save error: %v", err)
		}

		cfg2, _ := config.Load("")
		cfg2.LLM.Model = "model-v2"
		if err := config.Save(configPath, cfg2); err != nil {
			t.Fatalf("second Save error: %v", err)
		}

		loaded, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if loaded.LLM.Model != "model-v2" {
			t.Errorf("loaded.LLM.Model = %q, want 'model-v2'", loaded.LLM.Model)
		}
	})

	t.Run("returns error when cfg is nil", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		if err := config.Save(configPath, nil); err == nil {
			t.Errorf("Save(path, nil) expected error, got nil")
		}
	})
}
