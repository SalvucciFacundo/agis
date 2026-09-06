package doctor

import (
	"context"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func TestDoctor_CheckToolSearch(t *testing.T) {
	t.Run("disabled returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolsConfig{
				ToolSearch: config.ToolSearchConfig{
					Enabled: false,
				},
			},
		}
		doc := New(cfg)
		res := doc.checkToolSearch(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if res.Name != "tool_search" {
			t.Errorf("name = %q, want 'tool_search'", res.Name)
		}
		if !strings.Contains(res.Message, "Dynamic tool search disabled") {
			t.Errorf("message = %q, want containing 'Dynamic tool search disabled'", res.Message)
		}
	})

	t.Run("enabled with threshold returns PASS", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolsConfig{
				ToolSearch: config.ToolSearchConfig{
					Enabled:   true,
					Threshold: 10,
				},
			},
		}
		doc := New(cfg)
		res := doc.checkToolSearch(context.Background())

		if res.Status != StatusPass {
			t.Errorf("status = %v, want %v", res.Status, StatusPass)
		}
		if !strings.Contains(res.Message, "Dynamic tool search enabled (threshold: 10)") {
			t.Errorf("message = %q, want containing 'Dynamic tool search enabled (threshold: 10)'", res.Message)
		}
	})

	t.Run("enabled with non-positive threshold returns WARN", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolsConfig{
				ToolSearch: config.ToolSearchConfig{
					Enabled:   true,
					Threshold: 0,
				},
			},
		}
		doc := New(cfg)
		res := doc.checkToolSearch(context.Background())

		if res.Status != StatusWarn {
			t.Errorf("status = %v, want %v", res.Status, StatusWarn)
		}
		if !strings.Contains(res.Message, "Tool search threshold is <= 0") {
			t.Errorf("message = %q, want containing 'Tool search threshold is <= 0'", res.Message)
		}
	})
}
