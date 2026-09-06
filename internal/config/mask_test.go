package config_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestMaskSecrets_LLMFallbacksAndAPIKeys(t *testing.T) {
	orig := &config.Config{
		LLM: config.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-primary-key",
			APIKeys:  []string{"sk-primary-1", "sk-primary-2"},
			Fallbacks: []config.LLMFallbackConfig{
				{
					Provider: "openrouter",
					Model:    "anthropic/claude-3.5-sonnet",
					APIKey:   "sk-or-fallback",
					APIKeys:  []string{"sk-or-1", "sk-or-2"},
					BaseURL:  "https://openrouter.ai/api/v1",
				},
				{
					Provider: "ollama",
					Model:    "llama3.2",
					BaseURL:  "http://localhost:11434/v1",
				},
			},
		},
	}

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.LLM.APIKey != "sk-primary-key" {
		t.Errorf("Original LLM.APIKey mutated: %q", orig.LLM.APIKey)
	}
	if len(orig.LLM.APIKeys) != 2 || orig.LLM.APIKeys[0] != "sk-primary-1" {
		t.Errorf("Original LLM.APIKeys mutated: %+v", orig.LLM.APIKeys)
	}
	if len(orig.LLM.Fallbacks) != 2 || orig.LLM.Fallbacks[0].APIKey != "sk-or-fallback" {
		t.Errorf("Original LLM.Fallbacks mutated: %+v", orig.LLM.Fallbacks)
	}
	if len(orig.LLM.Fallbacks[0].APIKeys) != 2 || orig.LLM.Fallbacks[0].APIKeys[0] != "sk-or-1" {
		t.Errorf("Original LLM.Fallbacks[0].APIKeys mutated: %+v", orig.LLM.Fallbacks[0].APIKeys)
	}

	// Verify masked copy has "[MASKED]"
	if masked.LLM.APIKey != "[MASKED]" {
		t.Errorf("Masked LLM.APIKey = %q, want '[MASKED]'", masked.LLM.APIKey)
	}
	for i, k := range masked.LLM.APIKeys {
		if k != "[MASKED]" {
			t.Errorf("Masked LLM.APIKeys[%d] = %q, want '[MASKED]'", i, k)
		}
	}
	if masked.LLM.Fallbacks[0].APIKey != "[MASKED]" {
		t.Errorf("Masked Fallbacks[0].APIKey = %q, want '[MASKED]'", masked.LLM.Fallbacks[0].APIKey)
	}
	for i, k := range masked.LLM.Fallbacks[0].APIKeys {
		if k != "[MASKED]" {
			t.Errorf("Masked Fallbacks[0].APIKeys[%d] = %q, want '[MASKED]'", i, k)
		}
	}
	// Fallback without keys remains unchanged
	if masked.LLM.Fallbacks[1].APIKey != "" {
		t.Errorf("Masked Fallbacks[1].APIKey = %q, want ''", masked.LLM.Fallbacks[1].APIKey)
	}
}

func TestMaskSecrets_WebProviders(t *testing.T) {
	orig, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	orig.Tools.Web.Enabled = true
	orig.Tools.Web.Providers.Brave.APIKey = "bsa-secret-1"
	orig.Tools.Web.Providers.BraveAPIKey = "bsa-secret-2"
	orig.Tools.Web.Providers.Tavily.APIKey = "tvly-secret-1"
	orig.Tools.Web.Providers.TavilyAPIKey = "tvly-secret-2"
	orig.Tools.Web.Providers.Searxng.BaseURL = "http://searxng.local"
	orig.Tools.Web.DefaultProvider = "brave"

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.Tools.Web.Providers.Brave.APIKey != "bsa-secret-1" {
		t.Errorf("Original Brave.APIKey mutated: %q", orig.Tools.Web.Providers.Brave.APIKey)
	}
	if orig.Tools.Web.Providers.BraveAPIKey != "bsa-secret-2" {
		t.Errorf("Original BraveAPIKey mutated: %q", orig.Tools.Web.Providers.BraveAPIKey)
	}
	if orig.Tools.Web.Providers.Tavily.APIKey != "tvly-secret-1" {
		t.Errorf("Original Tavily.APIKey mutated: %q", orig.Tools.Web.Providers.Tavily.APIKey)
	}
	if orig.Tools.Web.Providers.TavilyAPIKey != "tvly-secret-2" {
		t.Errorf("Original TavilyAPIKey mutated: %q", orig.Tools.Web.Providers.TavilyAPIKey)
	}

	// Verify masked copy has "[MASKED]"
	if masked.Tools.Web.Providers.Brave.APIKey != "[MASKED]" {
		t.Errorf("Masked Brave.APIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.Brave.APIKey)
	}
	if masked.Tools.Web.Providers.BraveAPIKey != "[MASKED]" {
		t.Errorf("Masked BraveAPIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.BraveAPIKey)
	}
	if masked.Tools.Web.Providers.Tavily.APIKey != "[MASKED]" {
		t.Errorf("Masked Tavily.APIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.Tavily.APIKey)
	}
	if masked.Tools.Web.Providers.TavilyAPIKey != "[MASKED]" {
		t.Errorf("Masked TavilyAPIKey = %q, want '[MASKED]'", masked.Tools.Web.Providers.TavilyAPIKey)
	}

	// Verify non-secret fields remain preserved
	if masked.Tools.Web.Providers.Searxng.BaseURL != "http://searxng.local" {
		t.Errorf("Masked Searxng.BaseURL = %q, want 'http://searxng.local'", masked.Tools.Web.Providers.Searxng.BaseURL)
	}
	if masked.Tools.Web.DefaultProvider != "brave" {
		t.Errorf("Masked DefaultProvider = %q, want 'brave'", masked.Tools.Web.DefaultProvider)
	}
}

func TestMaskSecrets_ServerAPIKey(t *testing.T) {
	orig := &config.Config{
		Server: config.ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8080,
			APIKey:  "sk-server-secret-key-12345",
		},
	}

	masked := config.MaskSecrets(orig)

	if orig.Server.APIKey != "sk-server-secret-key-12345" {
		t.Errorf("Original Server.APIKey mutated: %q", orig.Server.APIKey)
	}
	if masked.Server.APIKey != "[MASKED]" {
		t.Errorf("Masked Server.APIKey = %q, want '[MASKED]'", masked.Server.APIKey)
	}
	if masked.Server.Host != "127.0.0.1" || masked.Server.Port != 8080 {
		t.Errorf("Non-secret Server fields altered: %+v", masked.Server)
	}
}

func TestMaskSecrets_SlackAndWhatsApp(t *testing.T) {
	orig := &config.Config{
		Gateway: config.GatewayConfig{
			Enabled: true,
			Slack: config.SlackConfig{
				Enabled:       true,
				BotToken:      "xoxb-secret-slack-token",
				SigningSecret: "slack-secret-signing-key",
				AllowedUsers:  []string{"U12345"},
				ListenAddr:    ":3002",
			},
			WhatsApp: config.WhatsAppConfig{
				Enabled:       true,
				APIToken:      "wa-secret-api-token",
				PhoneNumberID: "1234567890",
				VerifyToken:   "wa-secret-verify-token",
				AppSecret:     "wa-secret-app-key",
				AllowedUsers:  []string{"+15551234567"},
				ListenAddr:    ":3003",
			},
		},
	}

	masked := config.MaskSecrets(orig)

	// Verify original is untouched
	if orig.Gateway.Slack.BotToken != "xoxb-secret-slack-token" {
		t.Errorf("Original Slack.BotToken mutated: %q", orig.Gateway.Slack.BotToken)
	}
	if orig.Gateway.Slack.SigningSecret != "slack-secret-signing-key" {
		t.Errorf("Original Slack.SigningSecret mutated: %q", orig.Gateway.Slack.SigningSecret)
	}
	if orig.Gateway.WhatsApp.APIToken != "wa-secret-api-token" {
		t.Errorf("Original WhatsApp.APIToken mutated: %q", orig.Gateway.WhatsApp.APIToken)
	}
	if orig.Gateway.WhatsApp.VerifyToken != "wa-secret-verify-token" {
		t.Errorf("Original WhatsApp.VerifyToken mutated: %q", orig.Gateway.WhatsApp.VerifyToken)
	}
	if orig.Gateway.WhatsApp.AppSecret != "wa-secret-app-key" {
		t.Errorf("Original WhatsApp.AppSecret mutated: %q", orig.Gateway.WhatsApp.AppSecret)
	}

	// Verify masked copy has "[MASKED]"
	if masked.Gateway.Slack.BotToken != "[MASKED]" {
		t.Errorf("Masked Slack.BotToken = %q, want '[MASKED]'", masked.Gateway.Slack.BotToken)
	}
	if masked.Gateway.Slack.SigningSecret != "[MASKED]" {
		t.Errorf("Masked Slack.SigningSecret = %q, want '[MASKED]'", masked.Gateway.Slack.SigningSecret)
	}
	if masked.Gateway.WhatsApp.APIToken != "[MASKED]" {
		t.Errorf("Masked WhatsApp.APIToken = %q, want '[MASKED]'", masked.Gateway.WhatsApp.APIToken)
	}
	if masked.Gateway.WhatsApp.VerifyToken != "[MASKED]" {
		t.Errorf("Masked WhatsApp.VerifyToken = %q, want '[MASKED]'", masked.Gateway.WhatsApp.VerifyToken)
	}
	if masked.Gateway.WhatsApp.AppSecret != "[MASKED]" {
		t.Errorf("Masked WhatsApp.AppSecret = %q, want '[MASKED]'", masked.Gateway.WhatsApp.AppSecret)
	}

	// Verify non-secret fields remain preserved
	if masked.Gateway.Slack.ListenAddr != ":3002" {
		t.Errorf("Masked Slack.ListenAddr = %q, want ':3002'", masked.Gateway.Slack.ListenAddr)
	}
	if len(masked.Gateway.Slack.AllowedUsers) != 1 || masked.Gateway.Slack.AllowedUsers[0] != "U12345" {
		t.Errorf("Masked Slack.AllowedUsers = %+v, want ['U12345']", masked.Gateway.Slack.AllowedUsers)
	}
	if masked.Gateway.WhatsApp.PhoneNumberID != "1234567890" {
		t.Errorf("Masked WhatsApp.PhoneNumberID = %q, want '1234567890'", masked.Gateway.WhatsApp.PhoneNumberID)
	}
	if masked.Gateway.WhatsApp.ListenAddr != ":3003" {
		t.Errorf("Masked WhatsApp.ListenAddr = %q, want ':3003'", masked.Gateway.WhatsApp.ListenAddr)
	}
}
