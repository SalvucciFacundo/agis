package core

import (
	"context"
	"time"
)

// Decision is a PolicyGuard verdict.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionAsk
)

// String renders the decision for storage and display.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "allow"
	case DecisionDeny:
		return "deny"
	default:
		return "ask"
	}
}

// Posture is the baseline restrictiveness of one backend. sandbox denies by
// default (fail closed), standard applies the allowlist and asks outside it,
// and full is session-only blanket trust that is never persisted.
type Posture string

const (
	PostureSandbox  Posture = "sandbox"
	PostureStandard Posture = "standard"
	PostureFull     Posture = "full"
)

// Scope resolves an ask decision: what an approval means over time.
type Scope string

const (
	ScopeOnce    Scope = "once"
	ScopeSession Scope = "session"
	ScopeAlways  Scope = "always"
	ScopeDeny    Scope = "deny"
)

// Policy rule categories.
const (
	CategoryCommands = "commands"
	CategoryFiles    = "files"
	CategoryNetwork  = "network"
)

// GuardRequest identifies one policy evaluation subject.
type GuardRequest struct {
	Backend  string // local, docker, ssh
	Category string // commands, files, network
	Subject  string // the command string / path / host being evaluated
}

// RuleView is one persisted policy rule for display and administration.
type RuleView struct {
	Category string
	Backend  string
	Pattern  string
	Action   string // allow | deny
}

// AuditEntry is one audited security-relevant event.
type AuditEntry struct {
	ID       int64
	Time     time.Time
	Backend  string
	Category string
	Subject  string
	Decision string // allow | deny | ask
	Scope    string // empty except ask resolutions
}

// PolicyGuard evaluates subjects against the effective policy. Implementations
// never fail evaluation: a broken store yields deny (fail closed). The brain
// sees only this interface — it cannot mutate policy.
type PolicyGuard interface {
	Evaluate(ctx context.Context, req GuardRequest) Decision
}

// PolicyAdmin mutates and inspects policy state. It is consumed by the CLI and
// TUI only; keeping it out of the brain's reach is the trust boundary made
// type-level (the model can never grant itself permissions).
type PolicyAdmin interface {
	SetRule(ctx context.Context, category, backend, pattern, action string) error
	RemoveRule(ctx context.Context, category, backend, pattern string) error
	Rules(ctx context.Context) ([]RuleView, error)
	SetTier(ctx context.Context, backend string, posture Posture) error
	AuditTail(ctx context.Context, n int) ([]AuditEntry, error)
	ResolveAsk(ctx context.Context, req GuardRequest, scope Scope) error
	ClearSessionGrants()
}
