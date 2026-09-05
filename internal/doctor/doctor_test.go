package doctor

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/policy"
	_ "modernc.org/sqlite"
)

func TestDoctor_AllPass(t *testing.T) {
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

	// 1. Create SOUL.md
	soulPath := filepath.Join(tmpDir, "SOUL.md")
	if err := os.WriteFile(soulPath, []byte("You are AGIS, a helpful autonomous AI agent.\n"), 0o600); err != nil {
		t.Fatalf("writing SOUL.md: %v", err)
	}

	// 2. Create valid skill
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o700); err != nil {
		t.Fatalf("creating skills dir: %v", err)
	}
	skillContent := `---
name: test-skill
trigger: test
description: A test skill for doctor verification.
---
# Test Skill
Instructions here.
`
	if err := os.WriteFile(filepath.Join(skillsDir, "test.md"), []byte(skillContent), 0o600); err != nil {
		t.Fatalf("writing skill: %v", err)
	}

	// 3. Create SQLite DB and initialize migrations
	dbPath := filepath.Join(tmpDir, "agis.db")
	repo, err := memory.NewRepository(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	_ = repo.Close()

	// 4. Create policy.yaml
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	store, err := policy.Load(policyPath)
	if err != nil {
		t.Fatalf("creating policy store: %v", err)
	}
	if err := store.SetTier(context.Background(), "local", "sandbox"); err != nil {
		t.Fatalf("setting policy tier: %v", err)
	}

	// 5. Create config.yaml
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
		t.Fatalf("writing config.yaml: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	doc := New(cfg, WithHTTPClient(ts.Client()), WithAgisHome(tmpDir), WithOllamaURL(ts.URL))
	report := doc.Run(context.Background())

	if report.HasFailures() {
		t.Errorf("expected no failures, got report: %+v", report)
		for _, r := range report.Results {
			if r.Status == StatusFail {
				t.Errorf("failing check: %s - %s (details: %v)", r.Name, r.Message, r.Details)
			}
		}
	}

	// Verify expected checks are present
	expectedChecks := []string{"config", "database", "soul", "skills", "policy", "llm", "tools", "web_tools"}
	for _, name := range expectedChecks {
		found := false
		for _, r := range report.Results {
			if r.Name == name {
				found = true
				if r.Status != StatusPass {
					t.Errorf("check %s: expected PASS, got %s (%s)", name, r.Status, r.Message)
				}
				break
			}
		}
		if !found {
			t.Errorf("missing expected check %q in report", name)
		}
	}
}

func TestDoctor_FailuresDetected(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	// Configure an unreachable LLM endpoint and missing SQLite DB in unwritable path
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "", // missing API key triggers fail
		},
		DB: config.DBConfig{
			Path: filepath.Join(tmpDir, "nonexistent-dir\x00invalid", "agis.db"),
		},
		MCP: config.MCPConfig{
			Enabled: true,
			Servers: map[string]config.MCPServerConfig{
				"bad-server": {
					Command: "nonexistent-binary-123456",
				},
			},
		},
	}

	doc := New(cfg, WithAgisHome(tmpDir))
	report := doc.Run(context.Background())

	if !report.HasFailures() {
		t.Errorf("expected failures in report, but HasFailures() was false")
	}

	// Verify database check failed
	dbCheck := report.Find("database")
	if dbCheck == nil || dbCheck.Status != StatusFail {
		t.Errorf("database check: expected FAIL, got %v", dbCheck)
	}

	// Verify LLM check failed
	llmCheck := report.Find("llm")
	if llmCheck == nil || llmCheck.Status != StatusFail {
		t.Errorf("llm check: expected FAIL, got %v", llmCheck)
	}

	// Verify MCP check failed
	mcpCheck := report.Find("mcp")
	if mcpCheck == nil || mcpCheck.Status != StatusFail {
		t.Errorf("mcp check: expected FAIL, got %v", mcpCheck)
	}
}

func TestDoctor_WarningsDetected(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	// Create valid DB
	dbPath := filepath.Join(tmpDir, "agis.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening sqlite: %v", err)
	}
	_ = db.Close()

	// Missing SOUL.md and loose config permissions should produce warnings
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("llm:\n  provider: ollama\n"), 0o666); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	doc := New(cfg, WithAgisHome(tmpDir))
	report := doc.Run(context.Background())

	if !report.HasWarnings() {
		t.Errorf("expected warnings in report, got none")
	}

	soulCheck := report.Find("soul")
	if soulCheck == nil || soulCheck.Status != StatusWarn {
		t.Errorf("soul check: expected WARN, got %v", soulCheck)
	}
}

func TestDoctor_OpenAI_Checks(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	// 1. OpenAI 401 Unauthorized
	tsAuthFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer tsAuthFail.Close()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-invalid-key",
		},
		DB: config.DBConfig{
			Path: filepath.Join(tmpDir, "agis.db"),
		},
	}

	doc := New(cfg, WithAgisHome(tmpDir), WithHTTPClient(tsAuthFail.Client()), WithOpenAIBaseURL(tsAuthFail.URL))
	report := doc.Run(context.Background())

	llmCheck := report.Find("llm")
	if llmCheck == nil || llmCheck.Status != StatusFail {
		t.Errorf("expected fail on 401, got %v", llmCheck)
	}

	// 2. OpenAI OK
	tsAuthOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer tsAuthOK.Close()

	docOK := New(cfg, WithAgisHome(tmpDir), WithHTTPClient(tsAuthOK.Client()), WithOpenAIBaseURL(tsAuthOK.URL))
	reportOK := docOK.Run(context.Background())
	llmCheckOK := reportOK.Find("llm")
	if llmCheckOK == nil || llmCheckOK.Status != StatusPass {
		t.Errorf("expected pass on 200, got %v", llmCheckOK)
	}
}

func TestDoctor_EmbeddingsAndMCP(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("AGIS_HOME", tmpDir)

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Model:    "llama3.2",
		},
		DB: config.DBConfig{
			Path: filepath.Join(tmpDir, "agis.db"),
		},
		Embeddings: config.EmbeddingsConfig{
			Enabled:    true,
			Provider:   "ollama",
			Model:      "nomic-embed-text",
			Dimensions: 768,
			BatchSize:  100,
		},
		MCP: config.MCPConfig{
			Enabled: true,
			Servers: map[string]config.MCPServerConfig{
				"sh-srv": {
					Command: "sh",
				},
				"disabled-srv": {
					Command:  "nonexistent-cmd",
					Disabled: true,
				},
				"sse-srv": {
					URL: "http://localhost:8080/sse",
				},
			},
		},
	}

	doc := New(cfg, WithAgisHome(tmpDir))
	report := doc.Run(context.Background())

	embCheck := report.Find("embeddings")
	if embCheck == nil || embCheck.Status != StatusPass {
		t.Errorf("embeddings check: expected PASS, got %v", embCheck)
	}

	mcpCheck := report.Find("mcp")
	if mcpCheck == nil || mcpCheck.Status != StatusPass {
		t.Errorf("mcp check: expected PASS, got %v", mcpCheck)
	}
}

func TestDoctor_FormatTerminalAndJSON(t *testing.T) {
	report := &Report{
		Summary: ReportSummary{
			Total:    3,
			Passed:   1,
			Warnings: 1,
			Failed:   1,
		},
		Results: []CheckResult{
			{Name: "config", Title: "Config", Status: StatusPass, Message: "Valid configuration", Duration: 10 * time.Millisecond},
			{Name: "database", Title: "Database", Status: StatusWarn, Message: "Unmigrated tables", Duration: 20 * time.Millisecond},
			{Name: "llm", Title: "LLM", Status: StatusFail, Message: "Connection refused", Duration: 30 * time.Millisecond},
		},
	}

	// Check JSON serialization
	jsonBytes, err := report.JSON()
	if err != nil {
		t.Fatalf("report.JSON() error: %v", err)
	}
	var unmarshaled Report
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("unmarshaling json report: %v", err)
	}
	if len(unmarshaled.Results) != 3 {
		t.Errorf("expected 3 results in unmarshaled json, got %d", len(unmarshaled.Results))
	}

	// Check Terminal formatting with and without colors
	textColored := report.Format(true)
	if textColored == "" {
		t.Errorf("expected colored formatted text, got empty")
	}
	textPlain := report.Format(false)
	if textPlain == "" {
		t.Errorf("expected plain formatted text, got empty")
	}
}
