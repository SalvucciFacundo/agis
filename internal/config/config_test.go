package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestLoad_MemoryDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Memory.LearningEnabled {
		t.Error("Memory.LearningEnabled = false, want default true")
	}
	if cfg.Memory.RecallLimit != 10 {
		t.Errorf("Memory.RecallLimit = %d, want 10", cfg.Memory.RecallLimit)
	}
	if cfg.Memory.NudgeEvery != 10 {
		t.Errorf("Memory.NudgeEvery = %d, want 10", cfg.Memory.NudgeEvery)
	}
	if cfg.Memory.CloseTimeout != 30*time.Second {
		t.Errorf("Memory.CloseTimeout = %v, want 30s", cfg.Memory.CloseTimeout)
	}
}

func TestLoad_MemoryPartialOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "memory:\n  recall_limit: 5\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Only the present key is overlaid; the rest keep their defaults.
	if cfg.Memory.RecallLimit != 5 {
		t.Errorf("Memory.RecallLimit = %d, want 5", cfg.Memory.RecallLimit)
	}
	if !cfg.Memory.LearningEnabled {
		t.Error("Memory.LearningEnabled = false, want default true for a partial block")
	}
	if cfg.Memory.NudgeEvery != 10 {
		t.Errorf("Memory.NudgeEvery = %d, want default 10", cfg.Memory.NudgeEvery)
	}
}

func TestLoad_MemoryExplicitOffValuesSurvive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "memory:\n  learning_enabled: false\n  nudge_every: 0\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Memory.LearningEnabled {
		t.Error("Memory.LearningEnabled = true, want the explicit false to survive")
	}
	if cfg.Memory.NudgeEvery != 0 {
		t.Errorf("Memory.NudgeEvery = %d, want the explicit 0 (nudging disabled)", cfg.Memory.NudgeEvery)
	}
}

func TestLoad_MemoryCloseTimeoutDurationString(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "memory:\n  close_timeout: 45s\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v (want yaml.v3 to decode a duration string)", err)
	}
	if cfg.Memory.CloseTimeout != 45*time.Second {
		t.Errorf("Memory.CloseTimeout = %v, want 45s", cfg.Memory.CloseTimeout)
	}
}

func TestLoad_MemoryEmptyValuesRestoreDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "memory:\n  recall_limit: 0\n  close_timeout: 0s\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Memory.RecallLimit != 10 {
		t.Errorf("Memory.RecallLimit = %d, want restored default 10", cfg.Memory.RecallLimit)
	}
	if cfg.Memory.CloseTimeout != 30*time.Second {
		t.Errorf("Memory.CloseTimeout = %v, want restored default 30s", cfg.Memory.CloseTimeout)
	}
}

func TestLoad_AgentAndSkillsDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Agent.EvolutionEnabled {
		t.Error("Agent.EvolutionEnabled = false, want default true")
	}
	if cfg.Agent.Personalities != nil && len(cfg.Agent.Personalities) != 0 {
		t.Errorf("Agent.Personalities = %v, want empty by default", cfg.Agent.Personalities)
	}
	if !cfg.Skills.Enabled {
		t.Error("Skills.Enabled = false, want default true")
	}
	if cfg.Skills.Dir != filepath.Join(home, "skills") {
		t.Errorf("Skills.Dir = %q, want %q", cfg.Skills.Dir, filepath.Join(home, "skills"))
	}
}

func TestLoad_AgentExplicitOffSurvives(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	writeConfig(t, home, "agent:\n  evolution_enabled: false\n  personalities:\n    mentor: be a mentor\nskills:\n  enabled: false\n  dir: /tmp/my-skills\n", 0o600)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Agent.EvolutionEnabled {
		t.Error("Agent.EvolutionEnabled = true, want explicit false to survive")
	}
	if cfg.Agent.Personalities["mentor"] != "be a mentor" {
		t.Errorf("Agent.Personalities = %v, want the custom preset", cfg.Agent.Personalities)
	}
	if cfg.Skills.Enabled {
		t.Error("Skills.Enabled = true, want explicit false to survive")
	}
	if cfg.Skills.Dir != "/tmp/my-skills" {
		t.Errorf("Skills.Dir = %q, want the explicit override", cfg.Skills.Dir)
	}
}

func TestLoad_ToolsDefaultsAndExplicit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Tools.Enabled {
		t.Error("Tools.Enabled = true, want default false (tools are opt-in)")
	}
	if cfg.Tools.Docker.Image != "alpine:3" {
		t.Errorf("Tools.Docker.Image = %q, want alpine:3 default", cfg.Tools.Docker.Image)
	}

	writeConfig(t, home, "tools:\n  enabled: true\n  docker:\n    enabled: true\n    image: debian:12\n  ssh:\n    enabled: true\n    host: vps.example\n    user: kuno\n    key_path: ~/.ssh/id_ed25519\n", 0o600)

	cfg, err = Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Tools.Enabled || !cfg.Tools.Docker.Enabled || cfg.Tools.Docker.Image != "debian:12" {
		t.Errorf("tools config = %+v, want enabled with custom image", cfg.Tools)
	}
	if !cfg.Tools.SSH.Enabled || cfg.Tools.SSH.Host != "vps.example" || cfg.Tools.SSH.User != "kuno" {
		t.Errorf("ssh config = %+v, want enabled remote host", cfg.Tools.SSH)
	}
}

func TestLoad_CronDefaultsAndExplicit(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantEnabled bool
		wantJobs    int
		checkJobs   func(t *testing.T, cfg *Config)
	}{
		{
			name:        "empty config has cron disabled by default",
			yaml:        "",
			wantEnabled: false,
			wantJobs:    0,
		},
		{
			name: "cron block present but disabled",
			yaml: `cron:
  enabled: false
  jobs:
    - name: "test-job"
      schedule: "@every 1h"
      prompt: "ping"
`,
			wantEnabled: false,
			wantJobs:    1,
		},
		{
			name: "cron enabled with full job and target configuration",
			yaml: `cron:
  enabled: true
  jobs:
    - name: "daily-health"
      schedule: "@every 1h"
      prompt: "Check system health"
      session_id: "cron-health"
      target:
        adapter: "telegram"
        recipient: "123456"
    - name: "weekly-summary"
      schedule: "0 8 * * 1"
      prompt: "Generate weekly summary"
`,
			wantEnabled: true,
			wantJobs:    2,
			checkJobs: func(t *testing.T, cfg *Config) {
				j1 := cfg.Cron.Jobs[0]
				if j1.Name != "daily-health" || j1.Schedule != "@every 1h" || j1.Prompt != "Check system health" || j1.SessionID != "cron-health" {
					t.Errorf("unexpected job 0: %+v", j1)
				}
				if j1.Target == nil || j1.Target.Adapter != "telegram" || j1.Target.Recipient != "123456" {
					t.Errorf("unexpected target for job 0: %+v", j1.Target)
				}

				j2 := cfg.Cron.Jobs[1]
				if j2.Name != "weekly-summary" || j2.Schedule != "0 8 * * 1" || j2.Prompt != "Generate weekly summary" {
					t.Errorf("unexpected job 1: %+v", j2)
				}
				if j2.Target != nil {
					t.Errorf("expected nil target for job 1, got %+v", j2.Target)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("AGIS_HOME", home)
			if tt.yaml != "" {
				writeConfig(t, home, tt.yaml, 0o600)
			}

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Cron.Enabled != tt.wantEnabled {
				t.Errorf("Cron.Enabled = %v, want %v", cfg.Cron.Enabled, tt.wantEnabled)
			}
			if len(cfg.Cron.Jobs) != tt.wantJobs {
				t.Errorf("len(Cron.Jobs) = %d, want %d", len(cfg.Cron.Jobs), tt.wantJobs)
			}
			if tt.checkJobs != nil {
				tt.checkJobs(t, cfg)
			}
		})
	}
}

func TestLoad_GatewayDefaultsAndExplicit(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantGtw    bool
		wantTg     bool
		wantTgTok  string
		wantTgLen  int
		wantDc     bool
		wantDcTok  string
		wantDcLen  int
	}{
		{
			name:    "empty config uses defaults",
			yaml:    "",
			wantGtw: false,
			wantTg:  false,
			wantDc:  false,
		},
		{
			name: "gateway block present but disabled",
			yaml: `gateway:
  enabled: false
  telegram:
    enabled: false
`,
			wantGtw: false,
			wantTg:  false,
			wantDc:  false,
		},
		{
			name: "gateway and telegram enabled only",
			yaml: `gateway:
  enabled: true
  telegram:
    enabled: true
    token: "tg-only"
    allowlist: ["111"]
`,
			wantGtw:   true,
			wantTg:    true,
			wantTgTok: "tg-only",
			wantTgLen: 1,
			wantDc:    false,
		},
		{
			name: "full gateway config",
			yaml: `gateway:
  enabled: true
  telegram:
    enabled: true
    token: "tg-token-123"
    allowlist:
      - "12345"
      - "67890"
  discord:
    enabled: true
    token: "dc-token-456"
    allowlist:
      - "98765"
`,
			wantGtw:   true,
			wantTg:    true,
			wantTgTok: "tg-token-123",
			wantTgLen: 2,
			wantDc:    true,
			wantDcTok: "dc-token-456",
			wantDcLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("AGIS_HOME", home)
			if tt.yaml != "" {
				writeConfig(t, home, tt.yaml, 0o600)
			}

			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Gateway.Enabled != tt.wantGtw {
				t.Errorf("Gateway.Enabled = %v, want %v", cfg.Gateway.Enabled, tt.wantGtw)
			}
			if cfg.Gateway.Telegram.Enabled != tt.wantTg {
				t.Errorf("Gateway.Telegram.Enabled = %v, want %v", cfg.Gateway.Telegram.Enabled, tt.wantTg)
			}
			if cfg.Gateway.Telegram.Token != tt.wantTgTok {
				t.Errorf("Gateway.Telegram.Token = %q, want %q", cfg.Gateway.Telegram.Token, tt.wantTgTok)
			}
			if len(cfg.Gateway.Telegram.Allowlist) != tt.wantTgLen {
				t.Errorf("len(Gateway.Telegram.Allowlist) = %d, want %d", len(cfg.Gateway.Telegram.Allowlist), tt.wantTgLen)
			}
			if cfg.Gateway.Discord.Enabled != tt.wantDc {
				t.Errorf("Gateway.Discord.Enabled = %v, want %v", cfg.Gateway.Discord.Enabled, tt.wantDc)
			}
			if cfg.Gateway.Discord.Token != tt.wantDcTok {
				t.Errorf("Gateway.Discord.Token = %q, want %q", cfg.Gateway.Discord.Token, tt.wantDcTok)
			}
			if len(cfg.Gateway.Discord.Allowlist) != tt.wantDcLen {
				t.Errorf("len(Gateway.Discord.Allowlist) = %d, want %d", len(cfg.Gateway.Discord.Allowlist), tt.wantDcLen)
			}
		})
	}
}

func TestLoad_PluginsDefaultsAndExplicit(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantEnabled bool
		wantDir     string
	}{
		{
			name:        "empty config has plugins disabled with default dir",
			yaml:        "",
			wantEnabled: false,
			wantDir:     "",
		},
		{
			name: "explicit plugins config",
			yaml: `plugins:
  enabled: true
  dir: "/custom/plugins/dir"
`,
			wantEnabled: true,
			wantDir:     "/custom/plugins/dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("AGIS_HOME", home)
			path := writeConfig(t, home, tt.yaml, 0o600)

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Plugins.Enabled != tt.wantEnabled {
				t.Errorf("Plugins.Enabled = %v, want %v", cfg.Plugins.Enabled, tt.wantEnabled)
			}
			expectedDir := tt.wantDir
			if expectedDir == "" {
				expectedDir = filepath.Join(home, "plugins")
			}
			if cfg.Plugins.Dir != expectedDir {
				t.Errorf("Plugins.Dir = %q, want %q", cfg.Plugins.Dir, expectedDir)
			}
		})
	}
}

func TestLoad_WebhookDefaultsAndExplicit(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantEnabled bool
		wantHost    string
		wantPort    int
		wantPath    string
		wantSecret  string
		wantSession string
		wantTarget  *WebhookTargetConfig
	}{
		{
			name:        "empty config has webhook disabled with defaults",
			yaml:        "",
			wantEnabled: false,
			wantHost:    "127.0.0.1",
			wantPort:    8080,
			wantPath:    "/webhook",
			wantSecret:  "",
			wantSession: "webhook-events",
			wantTarget:  nil,
		},
		{
			name: "explicit webhook config",
			yaml: `webhook:
  enabled: true
  host: "0.0.0.0"
  port: 9000
  path: "/events"
  secret: "super-secret"
  default_session_id: "custom-webhook-session"
  target:
    adapter: "telegram"
    recipient: "78910"
`,
			wantEnabled: true,
			wantHost:    "0.0.0.0",
			wantPort:    9000,
			wantPath:    "/events",
			wantSecret:  "super-secret",
			wantSession: "custom-webhook-session",
			wantTarget: &WebhookTargetConfig{
				Adapter:   "telegram",
				Recipient: "78910",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("AGIS_HOME", home)
			path := writeConfig(t, home, tt.yaml, 0o600)

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Webhook.Enabled != tt.wantEnabled {
				t.Errorf("Webhook.Enabled = %v, want %v", cfg.Webhook.Enabled, tt.wantEnabled)
			}
			if cfg.Webhook.Host != tt.wantHost {
				t.Errorf("Webhook.Host = %q, want %q", cfg.Webhook.Host, tt.wantHost)
			}
			if cfg.Webhook.Port != tt.wantPort {
				t.Errorf("Webhook.Port = %d, want %d", cfg.Webhook.Port, tt.wantPort)
			}
			if cfg.Webhook.Path != tt.wantPath {
				t.Errorf("Webhook.Path = %q, want %q", cfg.Webhook.Path, tt.wantPath)
			}
			if cfg.Webhook.Secret != tt.wantSecret {
				t.Errorf("Webhook.Secret = %q, want %q", cfg.Webhook.Secret, tt.wantSecret)
			}
			if cfg.Webhook.DefaultSessionID != tt.wantSession {
				t.Errorf("Webhook.DefaultSessionID = %q, want %q", cfg.Webhook.DefaultSessionID, tt.wantSession)
			}
			if tt.wantTarget == nil {
				if cfg.Webhook.Target != nil {
					t.Errorf("Webhook.Target expected nil, got %+v", cfg.Webhook.Target)
				}
			} else {
				if cfg.Webhook.Target == nil {
					t.Fatalf("Webhook.Target expected %+v, got nil", tt.wantTarget)
				}
				if *cfg.Webhook.Target != *tt.wantTarget {
					t.Errorf("Webhook.Target = %+v, want %+v", *cfg.Webhook.Target, *tt.wantTarget)
				}
			}
		})
	}
}

