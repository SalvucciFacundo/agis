package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// section identifies the active tab in the permissions panel.
type section int

const (
	sectionRules section = iota
	sectionPostures
	sectionPreview
	sectionAudit
)

var sectionNames = []string{"rules", "postures", "preview", "audit"}

// Panel is the /permisos sub-model. It owns the PolicyAdmin/PolicyGuard
// handles and renders four sections: rules grouped by category, per-backend
// postures, a live decision preview, and the audit tail.
type Panel struct {
	admin core.PolicyAdmin
	guard core.PolicyGuard

	section section
	cursor  int

	rules []core.RuleView
	audit []core.AuditEntry

	previewInput  textinput.Model
	previewResult string
}

// NewPanel returns a panel bound to the given policy handles. Either may be
// nil in tests, which disables the corresponding section.
func NewPanel(admin core.PolicyAdmin, guard core.PolicyGuard) *Panel {
	ti := textinput.New()
	ti.Placeholder = "type a command to preview"
	ti.Width = 40
	return &Panel{
		admin:        admin,
		guard:        guard,
		previewInput: ti,
	}
}

// Refresh reloads rules and audit tail from the store.
func (p *Panel) Refresh(ctx context.Context) error {
	if p.admin != nil {
		rules, err := p.admin.Rules(ctx)
		if err != nil {
			return err
		}
		p.rules = rules

		audit, err := p.admin.AuditTail(ctx, 50)
		if err != nil {
			return err
		}
		p.audit = audit
	}
	return nil
}

// ToggleRule flips the action of the rule at cursor between allow and deny.
func (p *Panel) ToggleRule(ctx context.Context) error {
	if len(p.rules) == 0 || p.cursor < 0 || p.cursor >= len(p.rules) {
		return fmt.Errorf("no rule selected")
	}
	r := p.rules[p.cursor]
	newAction := "allow"
	if r.Action == "allow" {
		newAction = "deny"
	}
	if err := p.admin.SetRule(ctx, r.Category, r.Backend, r.Pattern, newAction); err != nil {
		return err
	}
	return p.Refresh(ctx)
}

// RevokeRule removes the rule at cursor (used for always grants).
func (p *Panel) RevokeRule(ctx context.Context) error {
	if len(p.rules) == 0 || p.cursor < 0 || p.cursor >= len(p.rules) {
		return fmt.Errorf("no rule selected")
	}
	r := p.rules[p.cursor]
	if err := p.admin.RemoveRule(ctx, r.Category, r.Backend, r.Pattern); err != nil {
		return err
	}
	if p.cursor >= len(p.rules) {
		p.cursor = len(p.rules) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	return p.Refresh(ctx)
}

// Preview evaluates subject through the guard and stores the rendered result.
func (p *Panel) Preview(ctx context.Context, backend, category, subject string) string {
	if p.guard == nil {
		p.previewResult = "no guard wired"
		return p.previewResult
	}
	req := core.GuardRequest{Backend: backend, Category: category, Subject: subject}
	p.previewResult = p.guard.Evaluate(ctx, req).String()
	return p.previewResult
}

// Init implements tea.Model.
func (p *Panel) Init() tea.Cmd { return nil }

// closePanelMsg signals the parent Model to hide the panel.
type closePanelMsg struct{}

// Update handles navigation and actions inside the panel.
func (p *Panel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return p, func() tea.Msg { return closePanelMsg{} }
		case "tab":
			p.section = (p.section + 1) % section(len(sectionNames))
			p.cursor = 0
			return p, nil
		case "j", "down":
			p.cursor++
			if p.cursor >= p.maxCursor() {
				p.cursor = p.maxCursor() - 1
			}
			if p.cursor < 0 {
				p.cursor = 0
			}
			return p, nil
		case "k", "up":
			p.cursor--
			if p.cursor < 0 {
				p.cursor = 0
			}
			return p, nil
		case " ":
			if p.section == sectionRules {
				_ = p.ToggleRule(context.Background())
			} else if p.section == sectionPostures {
				_ = p.cyclePosture(context.Background())
			}
			return p, nil
		case "r":
			if p.section == sectionRules {
				_ = p.RevokeRule(context.Background())
			}
			return p, nil
		}
		// Delegate typing to preview input when in preview section.
		if p.section == sectionPreview {
			var cmd tea.Cmd
			p.previewInput, cmd = p.previewInput.Update(msg)
			subject := strings.TrimSpace(p.previewInput.Value())
			if subject != "" {
				p.Preview(context.Background(), "local", core.CategoryCommands, subject)
			} else {
				p.previewResult = ""
			}
			return p, cmd
		}
	}
	return p, nil
}

func (p *Panel) maxCursor() int {
	switch p.section {
	case sectionRules:
		return len(p.rules)
	case sectionPostures:
		return 3 // local, docker, ssh
	case sectionAudit:
		return len(p.audit)
	default:
		return 1
	}
}

func (p *Panel) cyclePosture(ctx context.Context) error {
	if p.admin == nil {
		return nil
	}
	backends := []string{"local", "docker", "ssh"}
	if p.cursor < 0 || p.cursor >= len(backends) {
		return nil
	}
	backend := backends[p.cursor]
	// Toggle sandbox <-> standard
	current := ""
	if tierer, ok := p.admin.(interface {
		Tier(context.Context, string) (core.Posture, error)
	}); ok {
		if t, err := tierer.Tier(ctx, backend); err == nil {
			current = string(t)
		}
	}
	next := core.PostureStandard
	if current == string(core.PostureStandard) {
		next = core.PostureSandbox
	}
	return p.admin.SetTier(ctx, backend, next)
}

// View renders the panel.
func (p *Panel) View() string {
	var b strings.Builder
	// Header tabs
	for i, name := range sectionNames {
		if section(i) == p.section {
			b.WriteString("[" + name + "] ")
		} else {
			b.WriteString(name + " ")
		}
	}
	b.WriteString("\n\n")

	switch p.section {
	case sectionRules:
		if len(p.rules) == 0 {
			b.WriteString("(no rules)\n")
		}
		for i, r := range p.rules {
			prefix := "  "
			if i == p.cursor {
				prefix = "> "
			}
			b.WriteString(fmt.Sprintf("%s%-8s %-6s %-20s %s\n", prefix, r.Category, r.Backend, r.Pattern, r.Action))
		}
		b.WriteString("\n[space] toggle  [r] revoke  [tab] switch  [q] close")
	case sectionPostures:
		backends := []string{"local", "docker", "ssh"}
		for i, backend := range backends {
			prefix := "  "
			if i == p.cursor {
				prefix = "> "
			}
			posture := "sandbox"
			if p.admin != nil {
				if tierer, ok := p.admin.(interface {
					Tier(context.Context, string) (core.Posture, error)
				}); ok {
					if t, err := tierer.Tier(context.Background(), backend); err == nil && t != "" {
						posture = string(t)
					}
				}
			}
			b.WriteString(fmt.Sprintf("%s%-7s %s\n", prefix, backend, posture))
		}
		b.WriteString("\n[space] cycle posture  [tab] switch  [q] close")
	case sectionPreview:
		b.WriteString(p.previewInput.View() + "\n")
		if p.previewResult != "" {
			b.WriteString("decision: " + p.previewResult + "\n")
		}
		b.WriteString("\n[tab] switch  [q] close")
	case sectionAudit:
		if len(p.audit) == 0 {
			b.WriteString("(no audit entries)\n")
		}
		for i, e := range p.audit {
			if i >= 10 {
				break
			}
			b.WriteString(fmt.Sprintf("%s %-6s %s\n", e.Decision, e.Backend, e.Subject))
		}
		b.WriteString("\n[tab] switch  [q] close")
	}
	return b.String()
}
