package gateway_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
)

func TestAutoDenyApprover(t *testing.T) {
	t.Run("returns ScopeDeny for any guard request", func(t *testing.T) {
		approver := gateway.NewAutoDenyApprover(nil)
		req := core.GuardRequest{
			Backend:  "local",
			Category: "commands",
			Subject:  "rm -rf /tmp/test",
		}
		scope := approver(context.Background(), req)
		if scope != core.ScopeDeny {
			t.Errorf("approver() = %v, want %v", scope, core.ScopeDeny)
		}
	})

	t.Run("logs denied attempt when logger is provided", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		approver := gateway.NewAutoDenyApprover(logger)

		req := core.GuardRequest{
			Backend:  "docker",
			Category: "network",
			Subject:  "curl https://example.com",
		}
		scope := approver(context.Background(), req)
		if scope != core.ScopeDeny {
			t.Errorf("approver() = %v, want %v", scope, core.ScopeDeny)
		}

		out := buf.String()
		if !strings.Contains(out, "auto-denied") && !strings.Contains(out, "policy") {
			t.Errorf("log output = %q, want it to mention auto-denied or policy", out)
		}
		if !strings.Contains(out, "docker") || !strings.Contains(out, "curl https://example.com") {
			t.Errorf("log output = %q, want request backend and subject", out)
		}
	})
}
