package policy

import (
	"context"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// readOnlyCommands are the first tokens considered read-only under the
// sandbox posture. Conservative by design: anything not listed is treated as
// potentially destructive.
var readOnlyCommands = map[string]bool{
	"cat": true, "date": true, "find": true, "grep": true, "head": true,
	"ls": true, "pwd": true, "tail": true, "which": true, "whoami": true,
}

// readOnlyGitSubcommands are the git invocations sandbox may allow.
var readOnlyGitSubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "branch": true,
}

// networkCommands are first tokens that reach the network; sandbox refuses
// them regardless of any rule (spec TLS-002).
var networkCommands = map[string]bool{
	"curl": true, "wget": true, "nc": true, "scp": true, "sftp": true,
	"ssh": true, "ftp": true, "ping": true,
}

// destructiveCommands are refused under sandbox even if a pattern matches.
var destructiveCommands = map[string]bool{
	"chmod": true, "chown": true, "dd": true, "mkfs": true, "mv": true,
	"reboot": true, "rm": true, "rmdir": true, "shutdown": true,
}

// grants holds session-scoped ask approvals keyed backend+category+subject.
// They live only in memory and die at ClearSessionGrants (session close).
type grants map[string]core.Scope

func grantKey(req core.GuardRequest) string {
	return req.Backend + "\x00" + req.Category + "\x00" + req.Subject
}

// Evaluate runs the POL-002 flow and audits every outcome: broken store
// denies (fail closed); session grants allow; posture sets the baseline with
// rules layered on top; deny always wins over allow; sandbox refuses
// network/destructive classes regardless of any rule.
func (s *Store) Evaluate(ctx context.Context, req core.GuardRequest) core.Decision {
	d := s.evaluate(req)
	s.record(ctx, req, d.String(), "")
	return d
}

func (s *Store) evaluate(req core.GuardRequest) core.Decision {
	if s.broken {
		return core.DecisionDeny
	}
	if s.grants[grantKey(req)] == core.ScopeSession {
		return core.DecisionAllow
	}

	posture := core.Posture(s.file.Tiers[req.Backend])
	if posture == "" {
		posture = core.PostureSandbox
	}

	var allowMatched, denyMatched bool
	for _, r := range s.file.Rules[req.Category] {
		if r.Backend != req.Backend || !matchPattern(req.Subject, r.Pattern) {
			continue
		}
		if r.Action == "deny" {
			denyMatched = true
		} else {
			allowMatched = true
		}
	}
	if denyMatched {
		return core.DecisionDeny
	}

	switch posture {
	case core.PostureFull:
		return core.DecisionAllow

	case core.PostureStandard:
		if allowMatched {
			return core.DecisionAllow
		}
		return core.DecisionAsk

	default: // sandbox
		if req.Category == core.CategoryNetwork {
			if req.Backend == "web" && allowMatched {
				return core.DecisionAllow
			}
			return core.DecisionDeny
		}
		if req.Category == core.CategoryCommands && networkCommand(req.Subject) {
			return core.DecisionDeny
		}
		if req.Category == core.CategoryCommands && destructiveCommand(req.Subject) {
			return core.DecisionDeny
		}
		if allowMatched {
			if strings.HasPrefix(req.Backend, "mcp:") || req.Backend == "web" || req.Backend == "subagent" || readOnlySubject(req) {
				return core.DecisionAllow
			}
		}
		return core.DecisionDeny
	}
}

// ResolveAsk records an ask resolution per scope: session adds an in-memory
// grant, always persists an exact allow rule to policy.yaml, once and deny
// only audit.
func (s *Store) ResolveAsk(ctx context.Context, req core.GuardRequest, scope core.Scope) error {
	switch scope {
	case core.ScopeSession:
		s.grants[grantKey(req)] = core.ScopeSession
	case core.ScopeAlways:
		if err := s.SetRule(ctx, req.Category, req.Backend, req.Subject, "allow"); err != nil {
			return err
		}
	case core.ScopeOnce, core.ScopeDeny:
		// Nothing persistent; audited below.
	}
	s.record(ctx, req, "ask", string(scope))
	return nil
}

// ClearSessionGrants wipes every session-scoped approval.
func (s *Store) ClearSessionGrants() { s.grants = grants{} }

// networkCommand reports whether the command's first token reaches the net.
func networkCommand(subject string) bool {
	return tokenIn(subject, networkCommands)
}

// destructiveCommand reports whether the command's first token is destructive.
func destructiveCommand(subject string) bool {
	return tokenIn(subject, destructiveCommands)
}

// readOnlySubject classifies a subject as read-only for sandbox purposes.
func readOnlySubject(req core.GuardRequest) bool {
	switch req.Category {
	case core.CategoryFiles:
		return true // reads/writes on allowlisted paths are the sandbox's only file allowance
	case core.CategoryCommands:
		fields := strings.Fields(req.Subject)
		if len(fields) == 0 {
			return false
		}
		switch fields[0] {
		case "git":
			return len(fields) > 1 && readOnlyGitSubcommands[fields[1]]
		default:
			return readOnlyCommands[fields[0]]
		}
	default:
		return false
	}
}

// tokenIn reports whether the command's first token is in the set.
func tokenIn(subject string, set map[string]bool) bool {
	fields := strings.Fields(subject)
	if len(fields) == 0 {
		return false
	}
	return set[strings.ToLower(fields[0])]
}
