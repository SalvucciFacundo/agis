package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// recordedAudit captures every audit entry through the sink.
type recordedAudit struct {
	entries []core.AuditEntry
}

func (r *recordedAudit) AppendAudit(_ context.Context, e core.AuditEntry) error {
	r.entries = append(r.entries, e)
	return nil
}

func newTestStore(t *testing.T, content string) (*Store, *recordedAudit) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	s, err := Load(path)
	if err != nil && !s.Broken() {
		t.Fatalf("Load() error = %v but store not marked broken", err)
	}
	audit := &recordedAudit{}
	s.SetAuditSink(audit)
	return s, audit
}

func TestGuard_CorruptStoreFailsClosed(t *testing.T) {
	s, audit := newTestStore(t, "tiers: [unclosed\n")

	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "ls"}
	if got := s.Evaluate(context.Background(), req); got != core.DecisionDeny {
		t.Errorf("Evaluate = %v on corrupt store, want deny", got)
	}
	if !s.Broken() || s.Err() == nil {
		t.Error("broken state or error not exposed")
	}
	if len(audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1 deny row", len(audit.entries))
	}

	// Admin mutations must refuse while broken.
	if err := s.SetRule(context.Background(), core.CategoryCommands, "local", "ls", "allow"); err == nil {
		t.Error("SetRule succeeded on corrupt store, want refusal")
	}
	if err := s.SetTier(context.Background(), "local", core.PostureStandard); err == nil {
		t.Error("SetTier succeeded on corrupt store, want refusal")
	}
}

func TestGuard_SandboxDeniesByDefault(t *testing.T) {
	s, audit := newTestStore(t, "")
	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "anything"}
	if got := s.Evaluate(context.Background(), req); got != core.DecisionDeny {
		t.Errorf("sandbox Evaluate(anything) = %v, want deny", got)
	}
	if len(audit.entries) != 1 || audit.entries[0].Decision != "deny" {
		t.Errorf("audit = %+v, want one deny entry", audit.entries)
	}
}

func TestGuard_SandboxAllowsReadOnlyWithRule(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t, "")

	// Without a rule even read-only commands are denied...
	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "git status"}
	if got := s.Evaluate(ctx, req); got != core.DecisionDeny {
		t.Errorf("unallowlisted read-only = %v, want deny", got)
	}

	// ...with an allow rule they pass.
	if err := s.SetRule(ctx, core.CategoryCommands, "local", "git status", "allow"); err != nil {
		t.Fatalf("SetRule() error = %v", err)
	}
	if got := s.Evaluate(ctx, req); got != core.DecisionAllow {
		t.Errorf("allowlisted read-only = %v, want allow", got)
	}

	// But destructive commands stay denied even with a broad allow rule.
	if err := s.SetRule(ctx, core.CategoryCommands, "local", "rm", "allow"); err != nil {
		t.Fatalf("SetRule(rm) error = %v", err)
	}
	rm := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "rm -rf /tmp/x"}
	if got := s.Evaluate(ctx, rm); got != core.DecisionDeny {
		t.Errorf("destructive in sandbox = %v, want deny despite allow rule", got)
	}
}

func TestGuard_SandboxRefusesNetworkClass(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t, "")
	if err := s.SetRule(ctx, core.CategoryCommands, "local", "curl", "allow"); err != nil {
		t.Fatalf("SetRule(curl) error = %v", err)
	}

	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "curl example.com"}
	if got := s.Evaluate(ctx, req); got != core.DecisionDeny {
		t.Errorf("network command in sandbox = %v, want deny despite allow rule", got)
	}

	net := core.GuardRequest{Backend: "local", Category: core.CategoryNetwork, Subject: "example.com"}
	if got := s.Evaluate(ctx, net); got != core.DecisionDeny {
		t.Errorf("network category in sandbox = %v, want deny always", got)
	}
}

func TestGuard_StandardAsksOutsideAllowlist(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t, `
tiers:
  local: standard
rules:
  commands:
    - backend: local
      pattern: git
      action: allow
`)

	if got := s.Evaluate(ctx, core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "git push"}); got != core.DecisionAllow {
		t.Errorf("prefix match on allowlist = %v, want allow", got)
	}
	if got := s.Evaluate(ctx, core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "curl example.com"}); got != core.DecisionAsk {
		t.Errorf("outside allowlist = %v, want ask", got)
	}

	// Deny beats both prefix-allow and posture.
	if err := s.SetRule(ctx, core.CategoryCommands, "local", "git push --force", "deny"); err != nil {
		t.Fatalf("SetRule(deny) error = %v", err)
	}
	if got := s.Evaluate(ctx, core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "git push --force"}); got != core.DecisionDeny {
		t.Errorf("exact deny under prefix allow = %v, want deny", got)
	}
}

func TestGuard_FullIsBlanketSessionTrust(t *testing.T) {
	s, _ := newTestStore(t, "tiers:\n  local: full\n")
	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "whatever --dangerous"}

	if got := s.Evaluate(context.Background(), req); got != core.DecisionAllow {
		t.Errorf("full posture = %v, want allow", got)
	}

	// Full cannot be persisted via SetTier.
	if err := s.SetTier(context.Background(), "ssh", core.PostureFull); err == nil {
		t.Error("SetTier(full) accepted, want refusal")
	}
}

func TestGuard_SessionGrantSuppressesAskAndClears(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t, "tiers:\n  local: standard\n")
	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "make build"}

	if got := s.Evaluate(ctx, req); got != core.DecisionAsk {
		t.Fatalf("pre-grant = %v, want ask", got)
	}

	if err := s.ResolveAsk(ctx, req, core.ScopeSession); err != nil {
		t.Fatalf("ResolveAsk(session) error = %v", err)
	}
	if got := s.Evaluate(ctx, req); got != core.DecisionAllow {
		t.Errorf("post-grant = %v, want allow without asking", got)
	}

	s.ClearSessionGrants()
	if got := s.Evaluate(ctx, req); got != core.DecisionAsk {
		t.Errorf("post-clear = %v, want ask again after session close", got)
	}
}

func TestGuard_AlwaysGrantPersistsExactSubject(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t, "tiers:\n  local: sandbox\n")
	req := core.GuardRequest{Backend: "local", Category: core.CategoryCommands, Subject: "make build"}

	if err := s.ResolveAsk(ctx, req, core.ScopeAlways); err != nil {
		t.Fatalf("ResolveAsk(always) error = %v", err)
	}

	rules, err := s.Rules(ctx)
	if err != nil {
		t.Fatalf("Rules() error = %v", err)
	}
	found := false
	for _, r := range rules {
		if r.Pattern == "make build" && r.Action == "allow" {
			found = true
		}
	}
	if !found {
		t.Errorf("always grant did not persist exact-subject allow rule: %+v", rules)
	}

	// Sandbox still refuses non-read-only subjects even with the persisted rule.
	if got := s.Evaluate(ctx, req); got != core.DecisionDeny {
		t.Errorf("always-granted non-read-only in sandbox = %v, want deny", got)
	}
}

func TestMatchPattern_ExactPrefixPath(t *testing.T) {
	cases := []struct {
		subject, pattern string
		want             bool
	}{
		{"git status", "git", true},
		{"gitx", "git", false}, // prefix requires separator
		{"~/Documents/a.txt", "~/Documents", true},
		{"~/DocumentsX", "~/Documents", false},
		{"git", "git", true},
		{"curl example.com", "curl example.com", true},
	}
	for _, c := range cases {
		if got := matchPattern(c.subject, c.pattern); got != c.want {
			t.Errorf("matchPattern(%q,%q) = %v, want %v", c.subject, c.pattern, got, c.want)
		}
	}
}

func TestGuard_MCPEvaluation(t *testing.T) {
	ctx := context.Background()
	policyYAML := `
tiers:
  mcp:github: sandbox
  mcp:filesystem: standard
rules:
  commands:
    - backend: "mcp:github"
      pattern: "list_issues"
      action: "allow"
    - backend: "mcp:github"
      pattern: "delete_repo"
      action: "deny"
`
	s, _ := newTestStore(t, policyYAML)

	// 1. Sandbox posture: explicit allow rule allows
	allowReq := core.GuardRequest{Backend: "mcp:github", Category: core.CategoryCommands, Subject: "list_issues"}
	if got := s.Evaluate(ctx, allowReq); got != core.DecisionAllow {
		t.Errorf("sandbox MCP with allow rule = %v, want allow", got)
	}

	// 2. Sandbox posture: no matching rule denies
	unapprovedReq := core.GuardRequest{Backend: "mcp:github", Category: core.CategoryCommands, Subject: "create_issue"}
	if got := s.Evaluate(ctx, unapprovedReq); got != core.DecisionDeny {
		t.Errorf("sandbox MCP without rule = %v, want deny", got)
	}

	// 3. Explicit deny rule always denies
	denyReq := core.GuardRequest{Backend: "mcp:github", Category: core.CategoryCommands, Subject: "delete_repo"}
	if got := s.Evaluate(ctx, denyReq); got != core.DecisionDeny {
		t.Errorf("MCP with deny rule = %v, want deny", got)
	}

	// 4. Standard posture: unapproved tool triggers Ask
	stdReq := core.GuardRequest{Backend: "mcp:filesystem", Category: core.CategoryCommands, Subject: "read_file"}
	if got := s.Evaluate(ctx, stdReq); got != core.DecisionAsk {
		t.Errorf("standard MCP without rule = %v, want ask", got)
	}
}

func TestGuard_WebToolsEvaluation(t *testing.T) {
	ctx := context.Background()
	policyYAML := `
tiers:
  web: sandbox
rules:
  network:
    - backend: "web"
      pattern: "duckduckgo"
      action: "allow"
    - backend: "web"
      pattern: "https://evil.com"
      action: "deny"
`
	s, _ := newTestStore(t, policyYAML)

	// 1. Sandbox with allow rule for duckduckgo -> Allow
	allowReq := core.GuardRequest{Backend: "web", Category: core.CategoryNetwork, Subject: "duckduckgo"}
	if got := s.Evaluate(ctx, allowReq); got != core.DecisionAllow {
		t.Errorf("sandbox web with allow rule = %v, want allow", got)
	}

	// 2. Sandbox without rule -> Deny
	denyReq := core.GuardRequest{Backend: "web", Category: core.CategoryNetwork, Subject: "https://example.com"}
	if got := s.Evaluate(ctx, denyReq); got != core.DecisionDeny {
		t.Errorf("sandbox web without rule = %v, want deny", got)
	}

	// 3. Explicit deny rule -> Deny
	explicitDenyReq := core.GuardRequest{Backend: "web", Category: core.CategoryNetwork, Subject: "https://evil.com/payload"}
	if got := s.Evaluate(ctx, explicitDenyReq); got != core.DecisionDeny {
		t.Errorf("web with explicit deny rule = %v, want deny", got)
	}

	// 4. Standard posture: no rule -> Ask
	if err := s.SetTier(ctx, "web", core.PostureStandard); err != nil {
		t.Fatalf("SetTier error: %v", err)
	}
	stdReq := core.GuardRequest{Backend: "web", Category: core.CategoryNetwork, Subject: "https://golang.org"}
	if got := s.Evaluate(ctx, stdReq); got != core.DecisionAsk {
		t.Errorf("standard web without rule = %v, want ask", got)
	}

	// 5. Standard posture: allow rule -> Allow
	if got := s.Evaluate(ctx, allowReq); got != core.DecisionAllow {
		t.Errorf("standard web with allow rule = %v, want allow", got)
	}

	// 6. Full posture: blanket allow
	s.file.Tiers["web"] = "full"
	fullReq := core.GuardRequest{Backend: "web", Category: core.CategoryNetwork, Subject: "https://unknown-domain.com"}
	if got := s.Evaluate(ctx, fullReq); got != core.DecisionAllow {
		t.Errorf("full posture web = %v, want allow", got)
	}
}

func TestDecision_String(t *testing.T) {
	if core.DecisionAllow.String() != "allow" ||
		core.DecisionDeny.String() != "deny" ||
		core.DecisionAsk.String() != "ask" {
		t.Fatal("decision strings drifted")
	}
}

func TestGuard_SubagentEvaluation(t *testing.T) {
	ctx := context.Background()
	policyYAML := `
tiers:
  subagent: sandbox
rules:
  execution:
    - backend: "subagent"
      pattern: "allowed-task"
      action: "allow"
    - backend: "subagent"
      pattern: "forbidden-task"
      action: "deny"
`
	s, _ := newTestStore(t, policyYAML)

	// 1. Sandbox with allow rule -> Allow
	allowReq := core.GuardRequest{Backend: "subagent", Category: core.CategoryExecution, Subject: "allowed-task"}
	if got := s.Evaluate(ctx, allowReq); got != core.DecisionAllow {
		t.Errorf("sandbox subagent with allow rule = %v, want allow", got)
	}

	// 2. Sandbox without rule -> Deny
	unapprovedReq := core.GuardRequest{Backend: "subagent", Category: core.CategoryExecution, Subject: "random-task"}
	if got := s.Evaluate(ctx, unapprovedReq); got != core.DecisionDeny {
		t.Errorf("sandbox subagent without rule = %v, want deny", got)
	}

	// 3. Explicit deny rule -> Deny
	denyReq := core.GuardRequest{Backend: "subagent", Category: core.CategoryExecution, Subject: "forbidden-task"}
	if got := s.Evaluate(ctx, denyReq); got != core.DecisionDeny {
		t.Errorf("subagent with explicit deny rule = %v, want deny", got)
	}

	// 4. Standard posture without rule -> Ask
	if err := s.SetTier(ctx, "subagent", core.PostureStandard); err != nil {
		t.Fatalf("SetTier error: %v", err)
	}
	stdReq := core.GuardRequest{Backend: "subagent", Category: core.CategoryExecution, Subject: "unmatched-task"}
	if got := s.Evaluate(ctx, stdReq); got != core.DecisionAsk {
		t.Errorf("standard subagent without rule = %v, want ask", got)
	}

	// 5. Standard posture with allow rule -> Allow
	if got := s.Evaluate(ctx, allowReq); got != core.DecisionAllow {
		t.Errorf("standard subagent with allow rule = %v, want allow", got)
	}

	// 6. Full posture -> Allow
	s.SetTier(ctx, "subagent", core.PostureStandard) // clear and override
	s.ClearSessionGrants()
	// manually test posture full in memory
	fullStore, _ := newTestStore(t, "tiers:\n  subagent: full\n")
	fullReq := core.GuardRequest{Backend: "subagent", Category: core.CategoryExecution, Subject: "any-task"}
	if got := fullStore.Evaluate(ctx, fullReq); got != core.DecisionAllow {
		t.Errorf("full posture subagent = %v, want allow", got)
	}
}
