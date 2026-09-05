package setup_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/setup"
)

func TestWizard_NonInteractive_Ollama(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	wiz := setup.NewWizard(strings.NewReader(""), &stdout, &stderr)

	opts := setup.SetupOptions{
		Provider:       "ollama",
		Model:          "llama3.2",
		BaseURL:        ts.URL,
		NonInteractive: true,
		ConfigPath:     configPath,
	}

	exitCode := wiz.Run(opts)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", exitCode, stderr.String())
	}

	// Verify file was written and has 0600 permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("config file missing: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perm = %04o, want 0600", perm)
	}

	// Verify config content
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading saved config: %v", err)
	}
	if cfg.LLM.Provider != "ollama" || cfg.LLM.Model != "llama3.2" || cfg.LLM.BaseURL != ts.URL {
		t.Errorf("unexpected saved config: %+v", cfg.LLM)
	}
}

func TestWizard_NonInteractive_MissingAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	var stdout, stderr bytes.Buffer
	wiz := setup.NewWizard(strings.NewReader(""), &stdout, &stderr)

	opts := setup.SetupOptions{
		Provider:       "openai",
		Model:          "gpt-4o",
		NonInteractive: true,
		ConfigPath:     configPath,
	}

	exitCode := wiz.Run(opts)
	if exitCode != 2 {
		t.Errorf("expected exit code 2 for missing API key, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "-api-key is required") {
		t.Errorf("expected missing api key warning in stderr, got: %s", stderr.String())
	}
}

func TestWizard_NonInteractive_OverwriteProtection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("existing: true"), 0o600); err != nil {
		t.Fatalf("writing existing config: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	wiz := setup.NewWizard(strings.NewReader(""), &stdout, &stderr)

	opts := setup.SetupOptions{
		Provider:       "ollama",
		Model:          "llama3.2",
		BaseURL:        ts.URL,
		NonInteractive: true,
		Force:          false,
		ConfigPath:     configPath,
	}

	// Without force, should fail
	exitCode := wiz.Run(opts)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 when config exists without force, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected already exists error in stderr, got: %s", stderr.String())
	}

	// With force, should overwrite
	opts.Force = true
	exitCode = wiz.Run(opts)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 when force is true, got %d (stderr: %s)", exitCode, stderr.String())
	}
}

func TestWizard_NonInteractive_ProbeFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	wiz := setup.NewWizard(strings.NewReader(""), &stdout, &stderr)

	opts := setup.SetupOptions{
		Provider:       "openai",
		Model:          "gpt-4o",
		APIKey:         "sk-bad",
		BaseURL:        ts.URL,
		NonInteractive: true,
		Force:          false,
		ConfigPath:     configPath,
	}

	// Should fail probe and not write config
	exitCode := wiz.Run(opts)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 on probe failure, got %d", exitCode)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("config file should not have been created on probe failure")
	}

	// With force, should bypass probe failure and write config
	opts.Force = true
	exitCode = wiz.Run(opts)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 with force, got %d (stderr: %s)", exitCode, stderr.String())
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config file should have been written with force: %v", err)
	}
}

func TestWizard_Interactive_Flow(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	// Simulate interactive user input:
	// 1. Provider choice: "2" (openai)
	// 2. API key: "sk-interactive-test"
	// 3. Model: "" (accept default gpt-4o)
	// 4. Base URL: ts.URL
	userInput := strings.Join([]string{
		"openai",
		"sk-interactive-test",
		"",
		ts.URL,
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	wiz := setup.NewWizard(strings.NewReader(userInput), &stdout, &stderr)

	opts := setup.SetupOptions{
		NonInteractive: false,
		ConfigPath:     configPath,
	}

	exitCode := wiz.Run(opts)
	if exitCode != 0 {
		t.Fatalf("expected interactive exit code 0, got %d (stderr: %s)", exitCode, stderr.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading saved config: %v", err)
	}
	if cfg.LLM.Provider != "openai" || cfg.LLM.APIKey != "sk-interactive-test" || cfg.LLM.Model != "gpt-4o" || cfg.LLM.BaseURL != ts.URL {
		t.Errorf("unexpected interactive config: %+v", cfg.LLM)
	}
}
