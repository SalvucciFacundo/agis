// Package policy implements the Policy Guard: the single enforcement point
// for every tool call (spec §11). The store owns $AGIS_HOME/policy.yaml; the
// guard evaluates subjects against postures, rules, and session grants.
//
// Fail closed is the prime directive: a corrupt or unreadable policy file
// yields deny-all decisions with the error surfaced — never a permissive
// fallback.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"

	"gopkg.in/yaml.v3"
)

// ruleYAML is one stored rule.
type ruleYAML struct {
	Backend string `yaml:"backend"`
	Pattern string `yaml:"pattern"`
	Action  string `yaml:"action"`
}

// policyFile is the on-disk shape of policy.yaml.
type policyFile struct {
	Tiers map[string]string     `yaml:"tiers"`
	Rules map[string][]ruleYAML `yaml:"rules"`
}

// AuditSink receives one entry per security-relevant event. core.Repository
// satisfies it; failures are logged and never block a decision.
type AuditSink interface {
	AppendAudit(ctx context.Context, entry core.AuditEntry) error
}

// Store owns the policy file and implements both core ports: PolicyGuard for
// the brain and PolicyAdmin for CLI/TUI. It expects single-goroutine use.
type Store struct {
	path    string
	file    policyFile
	broken  bool
	loadErr error
	grants  map[string]core.Scope
	audit   AuditSink
}

// Load reads path, falling into deny-all mode when parsing fails. A missing
// file behaves as an empty policy (all tiers default to sandbox), which still
// denies everything by default.
func Load(path string) (*Store, error) {
	s := &Store{path: path, file: policyFile{
		Tiers: map[string]string{},
		Rules: map[string][]ruleYAML{},
	}, grants: map[string]core.Scope{}}

	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return s, nil
	case err != nil:
		s.broken, s.loadErr = true, fmt.Errorf("reading policy %s: %w", path, err)
		return s, s.loadErr
	}

	if err := yaml.Unmarshal(data, &s.file); err != nil {
		s.broken, s.loadErr = true, fmt.Errorf("parsing policy %s: %w", path, err)
		return s, s.loadErr
	}
	return s, nil
}

// SetAuditSink wires the audit destination. Nil disables auditing.
func (s *Store) SetAuditSink(sink AuditSink) { s.audit = sink }

// record writes one audit entry, logging (not returning) failures: a broken
// audit sink must never turn a decision into a crash.
func (s *Store) record(ctx context.Context, req core.GuardRequest, decision, scope string) {
	if s.audit == nil {
		return
	}
	entry := core.AuditEntry{
		Time:     time.Now().UTC(),
		Backend:  req.Backend,
		Category: req.Category,
		Subject:  req.Subject,
		Decision: decision,
		Scope:    scope,
	}
	if err := s.audit.AppendAudit(ctx, entry); err != nil {
		slog.Warn("policy: audit write failed", "error", err)
	}
}

// Err exposes why the store is in deny-all mode, if it is.
func (s *Store) Err() error { return s.loadErr }

// Broken reports whether the store failed to load and is denying everything.
func (s *Store) Broken() bool { return s.broken }

// save persists the current policy atomically (tmp + rename).
func (s *Store) save() error {
	data, err := yaml.Marshal(&s.file)
	if err != nil {
		return fmt.Errorf("marshaling policy: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("creating policy directory: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing policy tmp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("renaming policy into place: %w", err)
	}
	return nil
}

// matchPattern: exact subject, prefix-with-space (command arguments), or
// prefix-with-separator (paths). Deliberately not regex (design D2).
func matchPattern(subject, pattern string) bool {
	return subject == pattern ||
		strings.HasPrefix(subject, pattern+" ") ||
		strings.HasPrefix(subject, pattern+"/")
}

// SetRule adds or replaces one rule and persists.
func (s *Store) SetRule(_ context.Context, category, backend, pattern, action string) error {
	if s.broken {
		return fmt.Errorf("policy store is in deny-all mode: %w", s.loadErr)
	}
	if action != "allow" && action != "deny" {
		return fmt.Errorf("invalid action %q", action)
	}
	if category != core.CategoryCommands && category != core.CategoryFiles && category != core.CategoryNetwork {
		return fmt.Errorf("invalid category %q", category)
	}
	rules := s.file.Rules[category]
	for i, r := range rules {
		if r.Backend == backend && r.Pattern == pattern {
			rules[i].Action = action
			return s.save()
		}
	}
	s.file.Rules[category] = append(rules, ruleYAML{Backend: backend, Pattern: pattern, Action: action})
	return s.save()
}

// RemoveRule deletes every rule matching category/backend/pattern exactly.
func (s *Store) RemoveRule(_ context.Context, category, backend, pattern string) error {
	if s.broken {
		return fmt.Errorf("policy store is in deny-all mode: %w", s.loadErr)
	}
	rules := s.file.Rules[category]
	kept := rules[:0]
	for _, r := range rules {
		if r.Backend == backend && r.Pattern == pattern {
			continue
		}
		kept = append(kept, r)
	}
	s.file.Rules[category] = kept
	return s.save()
}

// Rules returns a flattened view of every stored rule.
func (s *Store) Rules(_ context.Context) ([]core.RuleView, error) {
	if s.broken {
		return nil, fmt.Errorf("policy store is in deny-all mode: %w", s.loadErr)
	}
	var out []core.RuleView
	for category, rules := range s.file.Rules {
		for _, r := range rules {
			out = append(out, core.RuleView{
				Category: category,
				Backend:  r.Backend,
				Pattern:  r.Pattern,
				Action:   r.Action,
			})
		}
	}
	return out, nil
}

// SetTier persists a baseline posture. Full is refused here: it is a
// session-only escalation managed through TUI approval (POL-004).
func (s *Store) SetTier(_ context.Context, backend string, posture core.Posture) error {
	if s.broken {
		return fmt.Errorf("policy store is in deny-all mode: %w", s.loadErr)
	}
	if posture == core.PostureFull {
		return fmt.Errorf("full posture is session-only; grant it through the TUI panel")
	}
	s.file.Tiers[backend] = string(posture)
	return s.save()
}
