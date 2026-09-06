package doctor

import (
	"context"
	"fmt"
	"time"
)

func (d *Doctor) checkSlackGateway(_ context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "gateway_slack",
		Title: "Slack Gateway",
	}

	cfg := d.cfg.Gateway.Slack
	if !cfg.Enabled {
		res.Status = StatusPass
		res.Message = "Slack gateway disabled"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.BotToken == "" {
		res.Status = StatusFail
		res.Message = "Slack bot token is missing"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.SigningSecret == "" {
		res.Status = StatusFail
		res.Message = "Slack signing secret is missing"
		res.Duration = time.Since(start)
		return res
	}

	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":3002"
	}
	res.Details = append(res.Details, fmt.Sprintf("Listen address: %s", addr))
	res.Details = append(res.Details, fmt.Sprintf("Allowed users: %d configured", len(cfg.AllowedUsers)))

	if len(cfg.AllowedUsers) == 0 {
		res.Status = StatusWarn
		res.Message = "Slack allowlist is empty (fail-closed, all messages will be rejected)"
		res.Duration = time.Since(start)
		return res
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("Slack gateway enabled (listen: %s, allowed users: %d)", addr, len(cfg.AllowedUsers))
	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkWhatsAppGateway(_ context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "gateway_whatsapp",
		Title: "WhatsApp Gateway",
	}

	cfg := d.cfg.Gateway.WhatsApp
	if !cfg.Enabled {
		res.Status = StatusPass
		res.Message = "WhatsApp gateway disabled"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.APIToken == "" {
		res.Status = StatusFail
		res.Message = "WhatsApp API token is missing"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.PhoneNumberID == "" {
		res.Status = StatusFail
		res.Message = "WhatsApp phone number ID is missing"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.VerifyToken == "" {
		res.Status = StatusFail
		res.Message = "WhatsApp verify token is missing"
		res.Duration = time.Since(start)
		return res
	}

	if cfg.AppSecret == "" {
		res.Status = StatusFail
		res.Message = "WhatsApp app secret is missing"
		res.Duration = time.Since(start)
		return res
	}

	addr := cfg.ListenAddr
	if addr == "" {
		addr = ":3003"
	}
	res.Details = append(res.Details, fmt.Sprintf("Listen address: %s", addr))
	res.Details = append(res.Details, fmt.Sprintf("Phone number ID: %s", cfg.PhoneNumberID))
	res.Details = append(res.Details, fmt.Sprintf("Allowed users: %d configured", len(cfg.AllowedUsers)))

	if len(cfg.AllowedUsers) == 0 {
		res.Status = StatusWarn
		res.Message = "WhatsApp allowlist is empty (fail-closed, all messages will be rejected)"
		res.Duration = time.Since(start)
		return res
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("WhatsApp gateway enabled (listen: %s, phone ID: %s)", addr, cfg.PhoneNumberID)
	res.Duration = time.Since(start)
	return res
}
