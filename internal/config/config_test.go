package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes cfg content to dir/config.yaml and returns the path.
func writeConfig(t *testing.T, dir, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoad_MissingFileUsesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.Provider != defaultProvider {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, defaultProvider)
	}
	if cfg.LLM.Model != defaultModel {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, defaultModel)
	}
	wantDB := filepath.Join(home, dbFileName)
	if cfg.DB.Path != wantDB {
		t.Errorf("DB.Path = %q, want %q", cfg.DB.Path, wantDB)
	}
}

func TestLoad_FlagOverridesAGISHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	// A config reachable only through the -config flag path.
	flagDir := t.TempDir()
	flagPath := writeConfig(t, flagDir, "llm:\n  provider: openai\n  model: gpt-4o-mini\n", 0o600)

	// A different config under AGIS_HOME that must be ignored.
	writeConfig(t, home, "llm:\n  provider: ollama\n  model: llama3.2\n", 0o600)

	cfg, err := Load(flagPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, "openai")
	}
	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "gpt-4o-mini")
	}
}

func TestLoad_AGISHomeOverridesDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "llm:\n  provider: ollama\n  model: custom-model\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.Model != "custom-model" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "custom-model")
	}
}

func TestLoad_PartialConfigKeepsDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "llm:\n  model: custom-model\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.Provider != defaultProvider {
		t.Errorf("LLM.Provider = %q, want default %q", cfg.LLM.Provider, defaultProvider)
	}
	if cfg.LLM.Model != "custom-model" {
		t.Errorf("LLM.Model = %q, want %q", cfg.LLM.Model, "custom-model")
	}
	if cfg.DB.Path != filepath.Join(home, dbFileName) {
		t.Errorf("DB.Path = %q, want %q", cfg.DB.Path, filepath.Join(home, dbFileName))
	}
}

func TestLoad_LoosePermissionsWarn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "llm:\n  provider: ollama\n", 0o644)

	var buf bytes.Buffer
	if _, err := Load("", WithWarnWriter(&buf)); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !strings.Contains(buf.String(), "0600") {
		t.Errorf("warning = %q, want it to mention expected 0600 perms", buf.String())
	}
}

func TestLoad_TightPermissionsNoWarn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "llm:\n  provider: ollama\n", 0o600)

	var buf bytes.Buffer
	if _, err := Load("", WithWarnWriter(&buf)); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("warning = %q, want none for 0600 perms", buf.String())
	}
}

func TestLoad_InvalidYAMLErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "llm: [unclosed\n", 0o600)

	if _, err := Load(""); err == nil {
		t.Fatal("Load() error = nil, want parse error")
	}
}

func TestResolvePath_Precedence(t *testing.T) {
	// Flag path wins over everything.
	if got := resolvePath("/explicit/config.yaml"); got != "/explicit/config.yaml" {
		t.Errorf("resolvePath(flag) = %q", got)
	}

	// AGIS_HOME wins over the default ~/.agis location.
	t.Setenv("AGIS_HOME", "/custom/home")
	if got := resolvePath(""); got != filepath.Join("/custom/home", configFileName) {
		t.Errorf("resolvePath(AGIS_HOME) = %q", got)
	}

	// Default falls back to ~/.agis/config.yaml.
	t.Setenv("AGIS_HOME", "")
	if got := resolvePath(""); !strings.HasSuffix(got, filepath.Join(dotAgisDir, configFileName)) {
		t.Errorf("resolvePath(default) = %q, want suffix %q", got, filepath.Join(dotAgisDir, configFileName))
	}
}
