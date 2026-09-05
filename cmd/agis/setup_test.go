package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestRunSetupCLI_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "help flag", args: []string{"--help"}},
		{name: "short help", args: []string{"-h"}},
		{name: "help command", args: []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var stdin bytes.Buffer
			code := RunSetupCLI(tt.args, &stdin, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Usage: agis setup") && !strings.Contains(stdout.String(), "agis init") {
				t.Errorf("stdout missing usage info: %s", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunSetupCLI_NonInteractive(t *testing.T) {
	t.Run("fails on invalid provider", func(t *testing.T) {
		var stdout, stderr, stdin bytes.Buffer
		code := RunSetupCLI([]string{"-non-interactive", "-provider", "unsupported"}, &stdin, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "invalid provider") {
			t.Errorf("stderr = %q, want invalid provider warning", stderr.String())
		}
	})

	t.Run("fails on missing required api key for cloud provider", func(t *testing.T) {
		var stdout, stderr, stdin bytes.Buffer
		code := RunSetupCLI([]string{"-non-interactive", "-provider", "openai"}, &stdin, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "-api-key is required") {
			t.Errorf("stderr = %q, want api-key required error", stderr.String())
		}
	})

	t.Run("fails when config exists and force is false", func(t *testing.T) {
		home := t.TempDir()
		cfgPath := filepath.Join(home, "config.yaml")
		if err := os.WriteFile(cfgPath, []byte("llm:\n  provider: ollama\n"), 0o600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		var stdout, stderr, stdin bytes.Buffer
		code := RunSetupCLI([]string{"-non-interactive", "-provider", "ollama", "-config", cfgPath}, &stdin, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "already exists") {
			t.Errorf("stderr = %q, want already exists error", stderr.String())
		}
	})

	t.Run("succeeds with force and custom config path", func(t *testing.T) {
		home := t.TempDir()
		cfgPath := filepath.Join(home, "config.yaml")

		var stdout, stderr, stdin bytes.Buffer
		code := RunSetupCLI([]string{
			"-non-interactive",
			"-provider", "openai",
			"-model", "gpt-4o",
			"-api-key", "sk-test-key-12345",
			"-config", cfgPath,
			"-force",
		}, &stdin, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "setup complete") && !strings.Contains(stdout.String(), "saved to") {
			t.Errorf("stdout = %q, want success confirmation", stdout.String())
		}

		// Verify file permissions 0600
		info, err := os.Stat(cfgPath)
		if err != nil {
			t.Fatalf("Stat error: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("config perm = %04o, want 0600", perm)
		}

		loaded, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if loaded.LLM.Provider != "openai" || loaded.LLM.Model != "gpt-4o" || loaded.LLM.APIKey != "sk-test-key-12345" {
			t.Errorf("loaded LLM mismatch: %+v", loaded.LLM)
		}
	})

	t.Run("succeeds with profile flag", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		var stdout, stderr, stdin bytes.Buffer
		code := RunSetupCLI([]string{
			"-non-interactive",
			"-provider", "ollama",
			"-model", "llama3.2",
			"-profile", "dev",
			"-force",
		}, &stdin, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}

		targetPath := filepath.Join(home, "profiles", "dev", "config.yaml")
		if _, err := os.Stat(targetPath); err != nil {
			t.Fatalf("expected profile config at %s, got: %v", targetPath, err)
		}
	})
}

func TestRunSetupCLI_InteractiveMock(t *testing.T) {
	home := t.TempDir()
	cfgPath := filepath.Join(home, "config.yaml")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{{"name": "llama3.2:latest"}},
		})
	}))
	defer ts.Close()

	// Provider selection: 1 (ollama)
	// API key: skipped for ollama
	// Model: llama3.2 (empty -> default)
	// Base URL: ts.URL
	userInput := strings.Join([]string{
		"1",
		"",
		ts.URL,
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	stdin := bytes.NewBufferString(userInput)

	code := RunSetupCLI([]string{"-config", cfgPath}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded.LLM.Provider != "ollama" {
		t.Errorf("loaded.LLM.Provider = %q, want ollama", loaded.LLM.Provider)
	}
}
