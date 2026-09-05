package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"gopkg.in/yaml.v3"
)

func TestRunConfigCLI_HelpAndUsage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "no arguments shows help on stdout with exit 0",
			args:       []string{},
			wantExit:   0,
			wantStdout: "Usage: agis config",
			wantStderr: "",
		},
		{
			name:       "help flag shows help on stdout with exit 0",
			args:       []string{"--help"},
			wantExit:   0,
			wantStdout: "Usage: agis config",
			wantStderr: "",
		},
		{
			name:       "help command shows help on stdout with exit 0",
			args:       []string{"help"},
			wantExit:   0,
			wantStdout: "Usage: agis config",
			wantStderr: "",
		},
		{
			name:       "unknown subcommand outputs error on stderr with exit 2",
			args:       []string{"invalid_cmd"},
			wantExit:   2,
			wantStdout: "",
			wantStderr: "unknown subcommand 'invalid_cmd'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := RunConfigCLI(tt.args, &stdout, &stderr)

			if code != tt.wantExit {
				t.Errorf("exit code = %d, want %d (stderr: %q)", code, tt.wantExit, stderr.String())
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunConfigCLI_Path(t *testing.T) {
	t.Run("default path resolution", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"path"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		wantPath := filepath.Join(home, "config.yaml") + "\n"
		if stdout.String() != wantPath {
			t.Errorf("stdout = %q, want %q", stdout.String(), wantPath)
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}
	})

	t.Run("custom path via -config flag", func(t *testing.T) {
		customPath := "/tmp/custom-agis/cfg.yaml"
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"path", "-config", customPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if stdout.String() != customPath+"\n" {
			t.Errorf("stdout = %q, want %q", stdout.String(), customPath+"\n")
		}
	})
}

func TestRunConfigCLI_Show(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg, _ := config.Load("")
	cfg.LLM.Provider = "openai"
	cfg.LLM.Model = "gpt-4o"
	cfg.LLM.APIKey = "sk-super-secret-12345"
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	t.Run("shows YAML with masked secrets by default", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"show", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}

		out := stdout.String()
		if !strings.Contains(out, "provider: openai") {
			t.Errorf("stdout missing 'provider: openai': %s", out)
		}
		if strings.Contains(out, "sk-super-secret-12345") {
			t.Errorf("stdout leaked secret API key: %s", out)
		}
		if !strings.Contains(out, "[MASKED]") {
			t.Errorf("stdout missing '[MASKED]' placeholder: %s", out)
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}
	})

	t.Run("shows plaintext secrets with -reveal flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"show", "-config", cfgPath, "-reveal"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}

		out := stdout.String()
		if !strings.Contains(out, "sk-super-secret-12345") {
			t.Errorf("stdout missing revealed secret: %s", out)
		}
	})

	t.Run("shows JSON output with -json flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"show", "-config", cfgPath, "-json"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}

		var parsed map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
			t.Fatalf("stdout is not valid JSON: %v (%s)", err, stdout.String())
		}

		llmBlock, ok := parsed["llm"].(map[string]any)
		if !ok {
			t.Fatalf("missing 'llm' block in JSON output")
		}
		if llmBlock["provider"] != "openai" {
			t.Errorf("llm.provider = %v, want openai", llmBlock["provider"])
		}
		if llmBlock["api_key"] != "[MASKED]" {
			t.Errorf("llm.api_key = %v, want [MASKED]", llmBlock["api_key"])
		}
	})

	t.Run("shows default config when file does not exist", func(t *testing.T) {
		missingPath := filepath.Join(tempDir, "missing.yaml")
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"show", "-config", missingPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "provider: ollama") {
			t.Errorf("stdout missing default provider ollama: %s", stdout.String())
		}
	})

	t.Run("corrupted YAML file outputs error on stderr with exit 1", func(t *testing.T) {
		corruptPath := filepath.Join(tempDir, "corrupt.yaml")
		if err := os.WriteFile(corruptPath, []byte("llm: [invalid yaml"), 0o600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"show", "-config", corruptPath}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if stderr.Len() == 0 {
			t.Errorf("expected error on stderr, got empty")
		}
	})
}

func TestRunConfigCLI_Get(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg, _ := config.Load("")
	cfg.LLM.Provider = "ollama"
	cfg.LLM.Model = "llama3.2"
	cfg.LLM.APIKey = "sk-secret-key-999"
	cfg.Memory.RecallLimit = 20
	cfg.Gateway.Telegram.Allowlist = []string{"admin", "mod"}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	t.Run("get scalar string value", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "llm.model", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if stdout.String() != "llama3.2\n" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "llama3.2\n")
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}
	})

	t.Run("get scalar int value", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "memory.recall_limit", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if stdout.String() != "20\n" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "20\n")
		}
	})

	t.Run("get sensitive key is masked by default", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "llm.api_key", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if stdout.String() != "[MASKED]\n" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "[MASKED]\n")
		}
	})

	t.Run("get sensitive key with -reveal", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "llm.api_key", "-config", cfgPath, "-reveal"}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if stdout.String() != "sk-secret-key-999\n" {
			t.Errorf("stdout = %q, want %q", stdout.String(), "sk-secret-key-999\n")
		}
	})

	t.Run("get complex type returns formatted YAML or JSON", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "gateway.telegram.allowlist", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		var list []string
		if err := yaml.Unmarshal(stdout.Bytes(), &list); err != nil {
			t.Fatalf("yaml unmarshal error: %v (%s)", err, stdout.String())
		}
		if len(list) != 2 || list[0] != "admin" || list[1] != "mod" {
			t.Errorf("list = %v, want [admin, mod]", list)
		}
	})

	t.Run("get unknown key returns error on stderr and exit 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "unknown.nonexistent.key", "-config", cfgPath}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "unknown configuration key") {
			t.Errorf("stderr = %q, want 'unknown configuration key'", stderr.String())
		}
	})

	t.Run("get missing positional argument returns usage error on stderr and exit 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"get", "-config", cfgPath}, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if stderr.Len() == 0 {
			t.Errorf("expected usage error on stderr, got empty")
		}
	})
}

func TestRunConfigCLI_Set(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	cfg, _ := config.Load("")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	t.Run("set valid string value updates file and prints confirmation", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"set", "llm.model", "llama3.3", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr: %q)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Updated 'llm.model' to 'llama3.3'") {
			t.Errorf("stdout = %q, want confirmation notice", stdout.String())
		}

		reloaded, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if reloaded.LLM.Model != "llama3.3" {
			t.Errorf("reloaded.LLM.Model = %q, want 'llama3.3'", reloaded.LLM.Model)
		}
	})

	t.Run("set valid bool value", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"set", "agent.evolution_enabled", "false", "-config", cfgPath}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		reloaded, _ := config.Load(cfgPath)
		if reloaded.Agent.EvolutionEnabled != false {
			t.Errorf("reloaded.Agent.EvolutionEnabled = true, want false")
		}
	})

	t.Run("set invalid type returns error on stderr, exit 1, leaves file untouched", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"set", "skills.enabled", "not_a_boolean", "-config", cfgPath}, &stdout, &stderr)

		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "invalid boolean value") {
			t.Errorf("stderr = %q, want 'invalid boolean value'", stderr.String())
		}
	})

	t.Run("set missing arguments returns usage error on stderr and exit 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunConfigCLI([]string{"set", "llm.model", "-config", cfgPath}, &stdout, &stderr)

		if code != 2 {
			t.Fatalf("exit code = %d, want 2", code)
		}
		if stderr.Len() == 0 {
			t.Errorf("expected usage error on stderr, got empty")
		}
	})
}
