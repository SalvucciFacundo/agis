package skills

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// DefaultMatchLimit is the number of skills Match returns when the caller
// does not bound it.
const DefaultMatchLimit = 3

// Hub is the in-memory skill index. It loads imported skills from a
// directory, keeps them synced to the repository, matches the current user
// input against name/trigger/description with whitespace-split AND term
// semantics (spec SKL-002), and tracks usage through the repository.
//
// The Hub expects single-goroutine use (the TUI loop), consistent with Brain;
// it carries no locking of its own.
type Hub struct {
	repo         core.Repository
	logger       *slog.Logger
	registryPath string

	skills []core.Skill
}

// NewHub returns an empty Hub backed by repo. registryPath may be empty to
// disable registry writes.
func NewHub(repo core.Repository, logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{repo: repo, logger: logger}
}

// LoadDir imports every valid skill file from dir, persists them as imported
// skills, and rebuilds the in-memory index from the repository so file skills
// and previously created agent skills are both visible.
func (h *Hub) LoadDir(ctx context.Context, dir string) error {
	fileSkills, err := LoadDir(dir, h.logger)
	if err != nil {
		return fmt.Errorf("loading skill directory: %w", err)
	}

	for _, s := range fileSkills {
		if err := h.repo.SaveSkill(ctx, s); err != nil {
			return fmt.Errorf("syncing imported skill %q: %w", s.Name, err)
		}
	}

	all, err := h.repo.ListSkills(ctx)
	if err != nil {
		return fmt.Errorf("refreshing skill index: %w", err)
	}
	h.skills = all
	return nil
}

// Match returns up to limit skills whose combined name, trigger, and
// description contain every whitespace-separated term of the input,
// case-insensitively. A non-positive limit falls back to DefaultMatchLimit.
func (h *Hub) Match(input string, limit int) []core.Skill {
	if limit <= 0 {
		limit = DefaultMatchLimit
	}

	terms := strings.Fields(strings.ToLower(input))
	if len(terms) == 0 {
		return nil
	}

	var out []core.Skill
	for _, s := range h.skills {
		haystack := strings.ToLower(s.Name + " " + s.Trigger + " " + s.Description)
		all := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				all = false
				break
			}
		}
		if all {
			out = append(out, s)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

// RecordUse marks a skill as used through the repository. Failures are logged
// and swallowed: usage tracking must never break a turn (spec SKL-003).
func (h *Hub) RecordUse(ctx context.Context, name string) {
	if err := h.repo.RecordSkillUsage(ctx, name); err != nil {
		h.logger.Warn("skills: recording usage failed", "name", name, "error", err)
	}
}

// Add indexes a freshly created agent skill that the caller already persisted,
// keeping the in-memory index current for the rest of the session.
func (h *Hub) Add(skill core.Skill) {
	for i, s := range h.skills {
		if s.Name == skill.Name {
			h.skills[i] = skill
			return
		}
	}
	h.skills = append(h.skills, skill)
}

// Skills returns the indexed skills in repository order (last_used DESC,
// then name). Callers treat it as read-only.
func (h *Hub) Skills() []core.Skill { return h.skills }
