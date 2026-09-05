package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/doctor"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/policy"
)

func TestRunDoctorCLI_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunDoctorCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("RunDoctorCLI(--help) = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "agis doctor") {
		t.Errorf("expected usage output in stdout, got %q", stdout.String())
	}
}

func TestRunDoctorCLI_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	// Setup fake Ollama server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "llama3.2:latest"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()
	t.Setenv("OLLAMA_HOST", ts.URL)

	// Create DB
	dbPath := filepath.Join(tmpDir, "agis.db")
	repo, err := memory.NewRepository(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("creating repo: %v", err)
	}
	_ = repo.Close()

	// Create policy
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	store, err := policy.Load(policyPath)
	if err != nil {
		t.Fatalf("creating policy: %v", err)
	}
	_ = store.SetTier(context.Background(), "local", "sandbox")

	// Create config
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `llm:
  provider: ollama
  model: llama3.2
db:
  path: ` + dbPath + `
mcp:
  enabled: false
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := RunDoctorCLI([]string{"-config", cfgPath, "-no-color"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("RunDoctorCLI() = %d, want 0; stderr: %s, stdout: %s", code, stderr.String(), stdout.String())
	}

	if !strings.Contains(stdout.String(), "AGIS System Health") {
		t.Errorf("expected report header in stdout, got %s", stdout.String())
	}
}

func TestRunDoctorCLI_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	dbPath := filepath.Join(tmpDir, "agis.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `llm:
  provider: ollama
db:
  path: ` + dbPath + `
`
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0o600)

	var stdout, stderr bytes.Buffer
	_ = RunDoctorCLI([]string{"-config", cfgPath, "-json"}, &stdout, &stderr)

	var report doctor.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected valid JSON report, failed to parse: %v; output: %s", err, stdout.String())
	}

	if report.Summary.Total == 0 {
		t.Errorf("expected checks in JSON report summary, got 0")
	}
}
