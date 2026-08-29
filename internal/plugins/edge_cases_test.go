package plugins_test

import (
	"context"
	"testing"

	"go.uber.org/goleak"

	"github.com/SalvucciFacundo/agis/internal/plugins"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestManager_EdgeCases(t *testing.T) {
	t.Run("nil logger and missing dir returns cleanly", func(t *testing.T) {
		mgr := plugins.NewManager(plugins.WithLogger(nil))
		err := mgr.Load("/path/that/does/not/exist/ever")
		if err != nil {
			t.Errorf("expected nil error on missing dir, got: %v", err)
		}
		if len(mgr.List()) != 0 {
			t.Errorf("expected 0 plugins, got %d", len(mgr.List()))
		}
	})

	t.Run("runner with missing entrypoint file errors gracefully", func(t *testing.T) {
		pluginsDir := t.TempDir()
		createTestPlugin(t, pluginsDir, "broken", `{
			"name": "broken",
			"version": "1.0.0",
			"entrypoint": "nonexistent-binary"
		}`, nil)

		mgr := plugins.NewManager(plugins.WithStateDir(pluginsDir))
		if err := mgr.Load(pluginsDir); err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if err := mgr.Enable("broken"); err != nil {
			t.Fatalf("Enable() error: %v", err)
		}

		runners := mgr.Runners()
		if len(runners) != 1 {
			t.Fatalf("expected 1 runner, got %d", len(runners))
		}

		ctx := context.Background()
		_, err := runners[0].Run(ctx, "test")
		if err == nil {
			t.Error("expected error running nonexistent entrypoint, got nil")
		}
	})
}
