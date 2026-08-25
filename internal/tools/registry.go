package tools

import (
	"log/slog"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// Select builds the enabled runners in policy-relevant order: local first,
// then docker, then ssh. Backends that are enabled but unavailable (missing
// binary, incomplete settings) are skipped with a logged warning — graceful
// degradation per spec TLS-001. An empty result means tools stay inert.
func Select(cfg config.ToolsConfig, logger *slog.Logger) []core.ToolRunner {
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Enabled {
		return nil
	}

	var out []core.ToolRunner

	// Local has no separate switch: tools.enabled governs it.
	out = append(out, NewLocal(0))

	if cfg.Docker.Enabled {
		if !available("docker") {
			logger.Warn("tools: docker backend enabled but docker binary not found; skipping")
		} else {
			out = append(out, NewDocker(cfg.Docker.Image))
		}
	}

	if cfg.SSH.Enabled {
		switch {
		case cfg.SSH.Host == "" || cfg.SSH.User == "":
			logger.Warn("tools: ssh backend enabled but host/user not configured; skipping")
		case !available("ssh"):
			logger.Warn("tools: ssh backend enabled but ssh binary not found; skipping")
		default:
			out = append(out, NewSSH(cfg.SSH.User, cfg.SSH.Host, cfg.SSH.KeyPath))
		}
	}

	return out
}

// available reports whether a binary is on PATH.
func available(bin string) bool {
	_, err := lookPath(bin)
	return err == nil
}
