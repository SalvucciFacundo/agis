package policy

import (
	"context"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestPolicyGuard_Subagent_SecurityTiers(t *testing.T) {
	ctx := context.Background()

	t.Run("sandbox posture blocks delegation without allow rule", func(t *testing.T) {
		s, audit := newTestStore(t, `
tiers:
  subagent: sandbox
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "analyze logs and summarize",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionDeny {
			t.Errorf("sandbox without rule: Evaluate() = %v, want %v", decision, core.DecisionDeny)
		}
		if len(audit.entries) != 1 || audit.entries[0].Decision != "deny" {
			t.Errorf("expected 1 deny audit entry, got: %+v", audit.entries)
		}
	})

	t.Run("sandbox posture permits delegation with allow rule", func(t *testing.T) {
		s, audit := newTestStore(t, `
tiers:
  subagent: sandbox
rules:
  execution:
    - backend: "subagent"
      pattern: "analyze logs"
      action: "allow"
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "analyze logs and summarize",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionAllow {
			t.Errorf("sandbox with matching allow rule: Evaluate() = %v, want %v", decision, core.DecisionAllow)
		}
		if len(audit.entries) != 1 || audit.entries[0].Decision != "allow" {
			t.Errorf("expected 1 allow audit entry, got: %+v", audit.entries)
		}
	})

	t.Run("standard posture returns DecisionAsk without matching rule", func(t *testing.T) {
		s, _ := newTestStore(t, `
tiers:
  subagent: standard
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "execute long task",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionAsk {
			t.Errorf("standard without rule: Evaluate() = %v, want %v", decision, core.DecisionAsk)
		}
	})

	t.Run("standard posture permits with allow rule", func(t *testing.T) {
		s, _ := newTestStore(t, `
tiers:
  subagent: standard
rules:
  execution:
    - backend: "subagent"
      pattern: "execute task"
      action: "allow"
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "execute task #42",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionAllow {
			t.Errorf("standard with matching rule: Evaluate() = %v, want %v", decision, core.DecisionAllow)
		}
	})

	t.Run("explicit deny rule takes precedence in all postures", func(t *testing.T) {
		s, _ := newTestStore(t, `
tiers:
  subagent: standard
rules:
  execution:
    - backend: "subagent"
      pattern: "dangerous"
      action: "deny"
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "dangerous recursive call",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionDeny {
			t.Errorf("deny rule: Evaluate() = %v, want %v", decision, core.DecisionDeny)
		}
	})

	t.Run("full posture permits delegation by default", func(t *testing.T) {
		s, _ := newTestStore(t, `
tiers:
  subagent: full
`)
		req := core.GuardRequest{
			Backend:  "subagent",
			Category: core.CategoryExecution,
			Subject:  "any random task",
		}

		decision := s.Evaluate(ctx, req)
		if decision != core.DecisionAllow {
			t.Errorf("full posture: Evaluate() = %v, want %v", decision, core.DecisionAllow)
		}
	})
}
