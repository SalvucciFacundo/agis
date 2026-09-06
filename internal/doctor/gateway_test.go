package doctor

import (
	"context"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestDoctor_CheckSlackGateway(t *testing.T) {
	t.Run("disabled returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				Slack: config.SlackConfig{Enabled: false},
			},
		}
		doc := New(cfg)
		res := doc.checkSlackGateway(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if res.Name != "gateway_slack" {
			t.Errorf("name = %q, want 'gateway_slack'", res.Name)
		}
		if !strings.Contains(res.Message, "Slack gateway disabled") {
			t.Errorf("message = %q, want containing 'Slack gateway disabled'", res.Message)
		}
	})

	t.Run("enabled with full config returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				Slack: config.SlackConfig{
					Enabled:       true,
					BotToken:      "xoxb-12345",
					SigningSecret: "secret-abc",
					AllowedUsers:  []string{"U12345"},
					ListenAddr:    ":3002",
				},
			},
		}
		doc := New(cfg)
		res := doc.checkSlackGateway(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if !strings.Contains(res.Message, "Slack gateway enabled") {
			t.Errorf("message = %q, want containing 'Slack gateway enabled'", res.Message)
		}
	})

	t.Run("missing bot token returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				Slack: config.SlackConfig{
					Enabled:       true,
					BotToken:      "",
					SigningSecret: "secret-abc",
					AllowedUsers:  []string{"U12345"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkSlackGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "Slack bot token is missing") {
			t.Errorf("message = %q, want containing 'Slack bot token is missing'", res.Message)
		}
	})

	t.Run("missing signing secret returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				Slack: config.SlackConfig{
					Enabled:       true,
					BotToken:      "xoxb-12345",
					SigningSecret: "",
					AllowedUsers:  []string{"U12345"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkSlackGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "Slack signing secret is missing") {
			t.Errorf("message = %q, want containing 'Slack signing secret is missing'", res.Message)
		}
	})

	t.Run("empty allowlist returns WARN", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				Slack: config.SlackConfig{
					Enabled:       true,
					BotToken:      "xoxb-12345",
					SigningSecret: "secret-abc",
					AllowedUsers:  nil,
				},
			},
		}
		doc := New(cfg)
		res := doc.checkSlackGateway(context.Background())

		if res.Status != StatusWarn {
			t.Errorf("status = %v, want %v", res.Status, StatusWarn)
		}
		if !strings.Contains(res.Message, "Slack allowlist is empty") {
			t.Errorf("message = %q, want containing 'Slack allowlist is empty'", res.Message)
		}
	})
}

func TestDoctor_CheckWhatsAppGateway(t *testing.T) {
	t.Run("disabled returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{Enabled: false},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if res.Name != "gateway_whatsapp" {
			t.Errorf("name = %q, want 'gateway_whatsapp'", res.Name)
		}
		if !strings.Contains(res.Message, "WhatsApp gateway disabled") {
			t.Errorf("message = %q, want containing 'WhatsApp gateway disabled'", res.Message)
		}
	})

	t.Run("enabled with full config returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "api-token-123",
					PhoneNumberID: "10987654321",
					VerifyToken:   "verify-token-xyz",
					AppSecret:     "app-secret-abc",
					AllowedUsers:  []string{"+15551234567"},
					ListenAddr:    ":3003",
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if !strings.Contains(res.Message, "WhatsApp gateway enabled") {
			t.Errorf("message = %q, want containing 'WhatsApp gateway enabled'", res.Message)
		}
	})

	t.Run("missing API token returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "",
					PhoneNumberID: "10987654321",
					VerifyToken:   "verify-token-xyz",
					AppSecret:     "app-secret-abc",
					AllowedUsers:  []string{"+15551234567"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "WhatsApp API token is missing") {
			t.Errorf("message = %q, want containing 'WhatsApp API token is missing'", res.Message)
		}
	})

	t.Run("missing phone number ID returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "api-token-123",
					PhoneNumberID: "",
					VerifyToken:   "verify-token-xyz",
					AppSecret:     "app-secret-abc",
					AllowedUsers:  []string{"+15551234567"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "WhatsApp phone number ID is missing") {
			t.Errorf("message = %q, want containing 'WhatsApp phone number ID is missing'", res.Message)
		}
	})

	t.Run("missing verify token returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "api-token-123",
					PhoneNumberID: "10987654321",
					VerifyToken:   "",
					AppSecret:     "app-secret-abc",
					AllowedUsers:  []string{"+15551234567"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "WhatsApp verify token is missing") {
			t.Errorf("message = %q, want containing 'WhatsApp verify token is missing'", res.Message)
		}
	})

	t.Run("missing app secret returns FAIL", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "api-token-123",
					PhoneNumberID: "10987654321",
					VerifyToken:   "verify-token-xyz",
					AppSecret:     "",
					AllowedUsers:  []string{"+15551234567"},
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusFail {
			t.Errorf("status = %v, want %v", res.Status, StatusFail)
		}
		if !strings.Contains(res.Message, "WhatsApp app secret is missing") {
			t.Errorf("message = %q, want containing 'WhatsApp app secret is missing'", res.Message)
		}
	})

	t.Run("empty allowlist returns WARN", func(t *testing.T) {
		cfg := &config.Config{
			Gateway: config.GatewayConfig{
				WhatsApp: config.WhatsAppConfig{
					Enabled:       true,
					APIToken:      "api-token-123",
					PhoneNumberID: "10987654321",
					VerifyToken:   "verify-token-xyz",
					AppSecret:     "app-secret-abc",
					AllowedUsers:  nil,
				},
			},
		}
		doc := New(cfg)
		res := doc.checkWhatsAppGateway(context.Background())

		if res.Status != StatusWarn {
			t.Errorf("status = %v, want %v", res.Status, StatusWarn)
		}
		if !strings.Contains(res.Message, "WhatsApp allowlist is empty") {
			t.Errorf("message = %q, want containing 'WhatsApp allowlist is empty'", res.Message)
		}
	})
}
