package config

import (
	"gopkg.in/yaml.v3"
)

const maskValue = "[MASKED]"

// MaskConfig is an alias for MaskSecrets for uniform API naming.
func MaskConfig(cfg *Config) *Config {
	return MaskSecrets(cfg)
}

// MaskSecrets returns a deep copy of cfg with sensitive credential fields obfuscated.
func MaskSecrets(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	// Deep copy via YAML roundtrip to safely clone all nested maps/slices
	data, err := yaml.Marshal(cfg)
	if err != nil {
		// Fallback: shallow copy if marshal fails
		clone := *cfg
		maskFields(&clone)
		return &clone
	}

	var clone Config
	if err := yaml.Unmarshal(data, &clone); err != nil {
		shallow := *cfg
		maskFields(&shallow)
		return &shallow
	}

	maskFields(&clone)
	return &clone
}

func maskFields(cfg *Config) {
	if cfg.LLM.APIKey != "" {
		cfg.LLM.APIKey = maskValue
	}
	for i := range cfg.LLM.APIKeys {
		cfg.LLM.APIKeys[i] = maskValue
	}
	for i := range cfg.LLM.Fallbacks {
		if cfg.LLM.Fallbacks[i].APIKey != "" {
			cfg.LLM.Fallbacks[i].APIKey = maskValue
		}
		for j := range cfg.LLM.Fallbacks[i].APIKeys {
			cfg.LLM.Fallbacks[i].APIKeys[j] = maskValue
		}
	}
	if cfg.Gateway.Telegram.Token != "" {
		cfg.Gateway.Telegram.Token = maskValue
	}
	if cfg.Gateway.Discord.Token != "" {
		cfg.Gateway.Discord.Token = maskValue
	}
	if cfg.Webhook.Secret != "" {
		cfg.Webhook.Secret = maskValue
	}
	if cfg.Tools.Web.Providers.Brave.APIKey != "" {
		cfg.Tools.Web.Providers.Brave.APIKey = maskValue
	}
	if cfg.Tools.Web.Providers.BraveAPIKey != "" {
		cfg.Tools.Web.Providers.BraveAPIKey = maskValue
	}
	if cfg.Tools.Web.Providers.Tavily.APIKey != "" {
		cfg.Tools.Web.Providers.Tavily.APIKey = maskValue
	}
	if cfg.Tools.Web.Providers.TavilyAPIKey != "" {
		cfg.Tools.Web.Providers.TavilyAPIKey = maskValue
	}
	if cfg.Server.APIKey != "" {
		cfg.Server.APIKey = maskValue
	}
}
