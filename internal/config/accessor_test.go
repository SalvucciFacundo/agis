package config_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		flagPath string
		agisHome string
		wantEnd  string
	}{
		{
			name:     "explicit flag path takes highest precedence",
			flagPath: "/custom/path/config.yaml",
			agisHome: "/env/home",
			wantEnd:  "/custom/path/config.yaml",
		},
		{
			name:     "AGIS_HOME environment variable when flag empty",
			flagPath: "",
			agisHome: "/env/home",
			wantEnd:  "/env/home/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGIS_HOME", tt.agisHome)
			got := config.ResolvePath(tt.flagPath)
			if got != tt.wantEnd {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.flagPath, got, tt.wantEnd)
			}
		})
	}
}

func TestGet(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load() unexpected error: %v", err)
	}

	cfg.LLM.Provider = "anthropic"
	cfg.LLM.Model = "claude-3-5-sonnet"
	cfg.LLM.APIKey = "sk-ant-12345"
	cfg.Tools.Docker.Image = "alpine:3.20"
	cfg.Memory.RecallLimit = 25
	cfg.Memory.CloseTimeout = 45 * time.Second
	cfg.Agent.EvolutionEnabled = true
	cfg.Gateway.Telegram.Allowlist = []string{"user1", "user2"}
	cfg.Tools.Web.Enabled = true
	cfg.Tools.Web.DefaultProvider = "brave"
	cfg.Tools.Web.FetchTimeout = 20 * time.Second
	cfg.Tools.Web.MaxFetchBytes = 4194304
	cfg.Tools.Web.Providers.Brave.APIKey = "bsa-key-accessor"
	cfg.Tools.Web.Providers.TavilyAPIKey = "tvly-key-accessor"

	tests := []struct {
		name      string
		key       string
		wantVal   any
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "get string scalar lower case",
			key:     "llm.provider",
			wantVal: "anthropic",
		},
		{
			name:    "get string scalar mixed case",
			key:     "LLM.Model",
			wantVal: "claude-3-5-sonnet",
		},
		{
			name:    "get nested string field",
			key:     "tools.docker.image",
			wantVal: "alpine:3.20",
		},
		{
			name:    "get int field",
			key:     "memory.recall_limit",
			wantVal: 25,
		},
		{
			name:    "get duration field",
			key:     "memory.close_timeout",
			wantVal: 45 * time.Second,
		},
		{
			name:    "get bool field",
			key:     "agent.evolution_enabled",
			wantVal: true,
		},
		{
			name:    "get slice field",
			key:     "gateway.telegram.allowlist",
			wantVal: []string{"user1", "user2"},
		},
		{
			name:    "get web tools default provider",
			key:     "tools.web.default_provider",
			wantVal: "brave",
		},
		{
			name:    "get web tools fetch timeout",
			key:     "tools.web.fetch_timeout",
			wantVal: 20 * time.Second,
		},
		{
			name:    "get web tools max fetch bytes",
			key:     "tools.web.max_fetch_bytes",
			wantVal: int64(4194304),
		},
		{
			name:    "get web tools nested brave api key",
			key:     "tools.web.providers.brave.api_key",
			wantVal: "bsa-key-accessor",
		},
		{
			name:    "get web tools flat tavily api key",
			key:     "tools.web.providers.tavily_api_key",
			wantVal: "tvly-key-accessor",
		},
		{
			name:    "get subagents enabled",
			key:     "subagents.enabled",
			wantVal: true,
		},
		{
			name:    "get subagents max concurrent",
			key:     "subagents.max_concurrent",
			wantVal: 3,
		},
		{
			name:    "get subagents default timeout",
			key:     "subagents.default_timeout",
			wantVal: 60 * time.Second,
		},
		{
			name:      "get unknown key returns error",
			key:       "invalid.unknown.key",
			wantErr:   true,
			errSubstr: "unknown configuration key",
		},
		{
			name:      "get empty key returns error",
			key:       "",
			wantErr:   true,
			errSubstr: "unknown configuration key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Get(cfg, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Get(%q) expected error, got nil", tt.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get(%q) unexpected error: %v", tt.key, err)
			}
			if !reflect.DeepEqual(got, tt.wantVal) {
				t.Errorf("Get(%q) = %v (%T), want %v (%T)", tt.key, got, got, tt.wantVal, tt.wantVal)
			}
		})
	}
}

func TestSet(t *testing.T) {
	t.Run("valid types mutate config", func(t *testing.T) {
		cfg, err := config.Load("")
		if err != nil {
			t.Fatalf("config.Load() error: %v", err)
		}

		if err := config.Set(cfg, "llm.provider", "openai"); err != nil {
			t.Fatalf("Set(llm.provider) error: %v", err)
		}
		if cfg.LLM.Provider != "openai" {
			t.Errorf("LLM.Provider = %q, want 'openai'", cfg.LLM.Provider)
		}

		if err := config.Set(cfg, "memory.recall_limit", "42"); err != nil {
			t.Fatalf("Set(memory.recall_limit) error: %v", err)
		}
		if cfg.Memory.RecallLimit != 42 {
			t.Errorf("Memory.RecallLimit = %d, want 42", cfg.Memory.RecallLimit)
		}

		if err := config.Set(cfg, "memory.close_timeout", "1m30s"); err != nil {
			t.Fatalf("Set(memory.close_timeout) error: %v", err)
		}
		if cfg.Memory.CloseTimeout != 90*time.Second {
			t.Errorf("Memory.CloseTimeout = %v, want 90s", cfg.Memory.CloseTimeout)
		}

		if err := config.Set(cfg, "agent.evolution_enabled", "false"); err != nil {
			t.Fatalf("Set(agent.evolution_enabled) error: %v", err)
		}
		if cfg.Agent.EvolutionEnabled != false {
			t.Errorf("Agent.EvolutionEnabled = true, want false")
		}

		if err := config.Set(cfg, "agent.evolution_enabled", "1"); err != nil {
			t.Fatalf("Set(agent.evolution_enabled) error: %v", err)
		}
		if cfg.Agent.EvolutionEnabled != true {
			t.Errorf("Agent.EvolutionEnabled = false, want true")
		}

		// Slice with comma-separated string
		if err := config.Set(cfg, "gateway.telegram.allowlist", "admin,ops,user"); err != nil {
			t.Fatalf("Set(gateway.telegram.allowlist comma) error: %v", err)
		}
		wantSlice := []string{"admin", "ops", "user"}
		if !reflect.DeepEqual(cfg.Gateway.Telegram.Allowlist, wantSlice) {
			t.Errorf("Gateway.Telegram.Allowlist = %v, want %v", cfg.Gateway.Telegram.Allowlist, wantSlice)
		}

		// Slice with JSON array string
		if err := config.Set(cfg, "gateway.discord.allowlist", `["123", "456"]`); err != nil {
			t.Fatalf("Set(gateway.discord.allowlist json) error: %v", err)
		}
		wantDiscord := []string{"123", "456"}
		if !reflect.DeepEqual(cfg.Gateway.Discord.Allowlist, wantDiscord) {
			t.Errorf("Gateway.Discord.Allowlist = %v, want %v", cfg.Gateway.Discord.Allowlist, wantDiscord)
		}

		// Web tools Set operations
		if err := config.Set(cfg, "tools.web.default_provider", "duckduckgo"); err != nil {
			t.Fatalf("Set(tools.web.default_provider) error: %v", err)
		}
		if cfg.Tools.Web.DefaultProvider != "duckduckgo" {
			t.Errorf("Tools.Web.DefaultProvider = %q, want duckduckgo", cfg.Tools.Web.DefaultProvider)
		}

		if err := config.Set(cfg, "tools.web.fetch_timeout", "25s"); err != nil {
			t.Fatalf("Set(tools.web.fetch_timeout) error: %v", err)
		}
		if cfg.Tools.Web.FetchTimeout != 25*time.Second {
			t.Errorf("Tools.Web.FetchTimeout = %v, want 25s", cfg.Tools.Web.FetchTimeout)
		}

		if err := config.Set(cfg, "tools.web.providers.brave.api_key", "new-bsa-key"); err != nil {
			t.Fatalf("Set(tools.web.providers.brave.api_key) error: %v", err)
		}
		if cfg.Tools.Web.Providers.Brave.APIKey != "new-bsa-key" {
			t.Errorf("Tools.Web.Providers.Brave.APIKey = %q, want new-bsa-key", cfg.Tools.Web.Providers.Brave.APIKey)
		}

		// Subagents Set operations
		if err := config.Set(cfg, "subagents.max_concurrent", "7"); err != nil {
			t.Fatalf("Set(subagents.max_concurrent) error: %v", err)
		}
		if cfg.Subagents.MaxConcurrent != 7 {
			t.Errorf("Subagents.MaxConcurrent = %d, want 7", cfg.Subagents.MaxConcurrent)
		}
		if err := config.Set(cfg, "subagents.default_timeout", "120s"); err != nil {
			t.Fatalf("Set(subagents.default_timeout) error: %v", err)
		}
		if cfg.Subagents.DefaultTimeout != 120*time.Second {
			t.Errorf("Subagents.DefaultTimeout = %v, want 120s", cfg.Subagents.DefaultTimeout)
		}
	})

	t.Run("invalid types and values reject without mutation", func(t *testing.T) {
		cfg, err := config.Load("")
		if err != nil {
			t.Fatalf("config.Load() error: %v", err)
		}

		cfg.Memory.RecallLimit = 10
		cfg.Agent.EvolutionEnabled = true

		if err := config.Set(cfg, "memory.recall_limit", "not_a_number"); err == nil {
			t.Errorf("Set int with non-number expected error, got nil")
		}
		if cfg.Memory.RecallLimit != 10 {
			t.Errorf("Memory.RecallLimit was mutated on error: got %d, want 10", cfg.Memory.RecallLimit)
		}

		if err := config.Set(cfg, "agent.evolution_enabled", "not_a_bool"); err == nil {
			t.Errorf("Set bool with non-bool expected error, got nil")
		}
		if cfg.Agent.EvolutionEnabled != true {
			t.Errorf("Agent.EvolutionEnabled was mutated on error")
		}

		if err := config.Set(cfg, "memory.close_timeout", "10lightyears"); err == nil {
			t.Errorf("Set duration with invalid duration expected error, got nil")
		}

		if err := config.Set(cfg, "unknown.field.name", "value"); err == nil {
			t.Errorf("Set unknown key expected error, got nil")
		}
	})
}

func TestMaskSecrets(t *testing.T) {
	orig, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	orig.LLM.APIKey = "sk-secret-llm"
	orig.Gateway.Telegram.Token = "tg-secret-token"
	orig.Gateway.Discord.Token = "dc-secret-token"
	orig.Webhook.Secret = "webhook-secret-key"
	orig.LLM.Provider = "openai"

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.LLM.APIKey != "sk-secret-llm" {
		t.Errorf("Original LLM.APIKey was mutated: %q", orig.LLM.APIKey)
	}
	if orig.Gateway.Telegram.Token != "tg-secret-token" {
		t.Errorf("Original Gateway.Telegram.Token was mutated: %q", orig.Gateway.Telegram.Token)
	}
	if orig.Gateway.Discord.Token != "dc-secret-token" {
		t.Errorf("Original Gateway.Discord.Token was mutated: %q", orig.Gateway.Discord.Token)
	}
	if orig.Webhook.Secret != "webhook-secret-key" {
		t.Errorf("Original Webhook.Secret was mutated: %q", orig.Webhook.Secret)
	}

	// Verify masked copy has "[MASKED]"
	if masked.LLM.APIKey != "[MASKED]" {
		t.Errorf("Masked LLM.APIKey = %q, want '[MASKED]'", masked.LLM.APIKey)
	}
	if masked.Gateway.Telegram.Token != "[MASKED]" {
		t.Errorf("Masked Gateway.Telegram.Token = %q, want '[MASKED]'", masked.Gateway.Telegram.Token)
	}
	if masked.Gateway.Discord.Token != "[MASKED]" {
		t.Errorf("Masked Gateway.Discord.Token = %q, want '[MASKED]'", masked.Gateway.Discord.Token)
	}
	if masked.Webhook.Secret != "[MASKED]" {
		t.Errorf("Masked Webhook.Secret = %q, want '[MASKED]'", masked.Webhook.Secret)
	}

	// Non-secret fields remain equal
	if masked.LLM.Provider != "openai" {
		t.Errorf("Masked LLM.Provider = %q, want 'openai'", masked.LLM.Provider)
	}
}
