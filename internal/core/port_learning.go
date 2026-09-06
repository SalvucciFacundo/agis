package core

import (
	"context"
	"fmt"
	"strings"
)

// SkillRef is one matched skill surfaced to the prompt assembly.
type SkillRef = Skill

// SkillHub is the consumer-side interface for the skill hub. It is satisfied
// by *skills.Hub; defining it here keeps core free of an adapter import.
// A nil SkillHub disables skill matching.
type SkillHub interface {
	// Match returns the skills relevant to the user's input, most relevant
	// first, at most limit entries.
	Match(input string, limit int) []Skill
	// Skills returns all indexed skills in repository order.
	Skills() []Skill
	// RecordUse marks a skill as used; implementations log-and-continue on
	// failure so tracking never breaks a turn.
	RecordUse(ctx context.Context, name string)
	// GetSkill retrieves a specific skill by name.
	GetSkill(name string) (*Skill, bool)
	// Reload scans dir, imports valid skills, and re-indexes the in-memory list.
	Reload(ctx context.Context, dir string) error
}

// EvolutionLayer supplies the derived persona guidance block (spec §8). It is
// satisfied by *persona.Evolution; a nil value disables the layer.
type EvolutionLayer interface {
	Layer(ctx context.Context) string
}

// SkillCreator distills agent-authored skills from a finished session. It is
// satisfied by *skills.Creator; a nil value disables close-time creation.
type SkillCreator interface {
	Extract(ctx context.Context, convID string, msgs []Message) (*Skill, error)
}

// defaultSkillMatchLimit caps how many skills enter context per turn.
const defaultSkillMatchLimit = 3

// maxToolRounds bounds the tool execution loop inside one Step (spec TOL-002).
// When reached, pending tool calls are dropped and audited, the model is told
// to answer directly, and the final round runs without tools advertised.
const maxToolRounds = 8

// ToolRunner executes approved commands on one backend.
type ToolRunner interface {
	Name() string
	Description() string
	Run(ctx context.Context, command string) (string, error)
	Backend() string
}

// toolDefs advertises available tools from registered runners.
func toolDefs(runners []ToolRunner) []ToolDef {
	defs := make([]ToolDef, 0, len(runners))
	for _, r := range runners {
		name := r.Name()
		if name == "" {
			name = "shell-" + r.Backend()
		}
		desc := r.Description()
		if desc == "" {
			desc = fmt.Sprintf(`Run a shell command on the %s backend. Arguments: {"command": "<the command string>"}. Prefer read-only commands.`, r.Backend())
		}
		defs = append(defs, ToolDef{
			Name:        name,
			Description: desc,
		})
	}
	return defs
}

// Approver resolves an ask decision interactively. Implementations block until
// the user answers; returning Deny (or an empty scope) blocks the action.
// Persisting session/always grants is the approver side's responsibility —
// never the brain's.
type Approver func(ctx context.Context, req GuardRequest) Scope

// skillsSystemMessage renders matched skills as one system message. When lazy is
// true, it renders a compact Markdown table index and tool usage instruction; when
// false, it renders the full Markdown bodies.
func skillsSystemMessage(matched []Skill, lazy bool) string {
	var b strings.Builder
	b.WriteString("Applicable skills for this request:\n")
	if !lazy {
		for _, s := range matched {
			fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Content)
		}
		return strings.TrimSuffix(b.String(), "\n")
	}

	b.WriteString("| Skill | Trigger / Description | Scope / Version |\n")
	b.WriteString("|---|---|---|\n")
	for _, s := range matched {
		desc := s.Description
		if s.Trigger != "" && s.Description != "" {
			desc = s.Trigger + " - " + s.Description
		} else if s.Trigger != "" {
			desc = s.Trigger
		}
		scope := s.Source
		if scope == "" {
			scope = "imported"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", s.Name, desc, scope)
	}
	b.WriteString("\nTo read complete procedural instructions, call read_skill(name)")
	return b.String()
}
