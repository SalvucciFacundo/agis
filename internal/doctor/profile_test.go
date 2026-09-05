package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestDoctor_CheckProfile(t *testing.T) {
	t.Run("default profile passes when home exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		cfg := &config.Config{}
		doc := New(cfg, WithAgisHome(home))
		report := doc.Run(context.Background())

		res := report.Find("profile")
		if res == nil {
			t.Fatal("expected profile check in doctor report")
		}
		if res.Status != StatusPass {
			t.Errorf("status = %s, want %s (msg: %s)", res.Status, StatusPass, res.Message)
		}
	})

	t.Run("named profile passes when directory and files exist", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		if err := config.CreateProfile("work", ""); err != nil {
			t.Fatalf("CreateProfile error: %v", err)
		}
		if err := config.SwitchProfile("work"); err != nil {
			t.Fatalf("SwitchProfile error: %v", err)
		}

		cfg := &config.Config{}
		doc := New(cfg, WithAgisHome(home))
		report := doc.Run(context.Background())

		res := report.Find("profile")
		if res == nil {
			t.Fatal("expected profile check in doctor report")
		}
		if res.Status != StatusPass {
			t.Errorf("status = %s, want %s (msg: %s)", res.Status, StatusPass, res.Message)
		}
	})

	t.Run("fails when active profile pointer points to missing directory", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		pointerPath := filepath.Join(home, ".active_profile")
		if err := os.WriteFile(pointerPath, []byte("ghost_profile\n"), 0o600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		cfg := &config.Config{}
		doc := New(cfg, WithAgisHome(home))
		report := doc.Run(context.Background())

		res := report.Find("profile")
		if res == nil {
			t.Fatal("expected profile check in doctor report")
		}
		if res.Status != StatusFail {
			t.Errorf("status = %s, want %s (msg: %s)", res.Status, StatusFail, res.Message)
		}
	})

	t.Run("warns when profile config file has loose permissions", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("AGIS_HOME", home)

		if err := config.CreateProfile("sec-test", ""); err != nil {
			t.Fatalf("CreateProfile error: %v", err)
		}
		if err := config.SwitchProfile("sec-test"); err != nil {
			t.Fatalf("SwitchProfile error: %v", err)
		}

		cfgPath := filepath.Join(home, "profiles", "sec-test", "config.yaml")
		if err := os.Chmod(cfgPath, 0o666); err != nil {
			t.Fatalf("Chmod error: %v", err)
		}

		cfg := &config.Config{}
		doc := New(cfg, WithAgisHome(home))
		report := doc.Run(context.Background())

		res := report.Find("profile")
		if res == nil {
			t.Fatal("expected profile check in doctor report")
		}
		if res.Status != StatusWarn {
			t.Errorf("status = %s, want %s (msg: %s)", res.Status, StatusWarn, res.Message)
		}
	})
}
