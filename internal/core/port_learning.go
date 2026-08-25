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
	// RecordUse marks a skill as used; implementations log-and-continue on
	// failure so tracking never breaks a turn.
	RecordUse(ctx context.Context, name string)
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

// skillsSystemMessage renders matched skills as one system message.
func skillsSystemMessage(matched []Skill) string {
	var b strings.Builder
	b.WriteString("Applicable skills for this request:\n")
	for _, s := range matched {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Content)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
