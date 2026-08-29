package gateway

import (
	"context"
	"log/slog"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// NewAutoDenyApprover returns a non-interactive Approver func for background
// daemons (Gateway, Cron, Webhook). It automatically denies any DecisionAsk
// (or tool request requiring escalation), preventing remote deadlock in non-interactive sessions.
func NewAutoDenyApprover(logger *slog.Logger) core.Approver {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, req core.GuardRequest) core.Scope {
		logger.Warn("gateway policy: auto-denied interactive tool request",
			"backend", req.Backend,
			"category", req.Category,
			"subject", req.Subject,
			"decision", "deny",
		)
		return core.ScopeDeny
	}
}
