package tools

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

func backendNames(rs []core.ToolRunner) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Backend()
	}
	return out
}

func TestSelect_LocalAlwaysFirstWhenEnabled(t *testing.T) {
	old := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/x", nil }
	defer func() { lookPath = old }()

	runners := Select(config.ToolsConfig{Enabled: true}, slog.New(slog.DiscardHandler))
	if len(runners) < 1 || runners[0].Backend() != "local" {
		t.Fatalf("runners[0] = %v, want local first", backendNames(runners))
	}
}

func TestSelect_SkipsEnabledButMissingBinaries(t *testing.T) {
	old := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	defer func() { lookPath = old }()

	cfg := config.ToolsConfig{
		Enabled: true,
		Docker:  config.DockerConfig{Enabled: true},
		SSH:     config.SSHConfig{Enabled: true, Host: "h", User: "u"},
	}
	backends := backendNames(Select(cfg, slog.New(slog.DiscardHandler)))
	if len(backends) != 1 || backends[0] != "local" {
		t.Errorf("backends = %v, want only local", backends)
	}
}

func TestSelect_DisabledToolsInert(t *testing.T) {
	if runners := Select(config.ToolsConfig{}, slog.New(slog.DiscardHandler)); len(runners) != 0 {
		t.Errorf("disabled config returned %d runners, want 0", len(runners))
	}
}

func TestSelect_SSHIncompleteSettingsSkips(t *testing.T) {
	old := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/ssh", nil }
	defer func() { lookPath = old }()

	cfg := config.ToolsConfig{Enabled: true, SSH: config.SSHConfig{Enabled: true}}
	for _, b := range backendNames(Select(cfg, slog.New(slog.DiscardHandler))) {
		if b == "ssh" {
			t.Error("ssh registered without host/user")
		}
	}
}

func TestSelect_FullStackOrder(t *testing.T) {
	old := lookPath
	lookPath = func(string) (string, error) { return "/usr/bin/bin", nil }
	defer func() { lookPath = old }()

	cfg := config.ToolsConfig{
		Enabled: true,
		Docker:  config.DockerConfig{Enabled: true},
		SSH:     config.SSHConfig{Enabled: true, Host: "h", User: "u"},
	}
	got := backendNames(Select(cfg, slog.New(slog.DiscardHandler)))
	want := []string{"local", "docker", "ssh"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("order[%d] = %q, want %q", i, got[i], w)
		}
	}
}
