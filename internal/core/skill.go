package core

import "time"

// Skill sources. Skills enter the system either imported from user-provided
// files or created by the agent from session experience.
const (
	// SourceImported marks a skill loaded from a user-provided file.
	SourceImported = "imported"
	// SourceAgent marks a skill created by the agent from experience.
	SourceAgent = "agent"
)

// Skill is one unit of procedural memory: reusable instructions for
// accomplishing a task. It is keyed by Name — re-saving a skill with the same
// name updates the existing row instead of duplicating it. UsageCount and
// LastUsed track how often the hub injected the skill into context; they are
// maintained by RecordSkillUsage, not by SaveSkill.
type Skill struct {
	ID          string
	Name        string
	Description string
	Trigger     string
	Content     string
	Source      string
	UsageCount  int
	LastUsed    time.Time
	CreatedAt   time.Time
}
