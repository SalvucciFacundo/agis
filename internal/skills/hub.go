package skills

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// DefaultMatchLimit is the number of skills Match returns when the caller
// does not bound it.
const DefaultMatchLimit = 3

// stopWords are dropped from the input before AND matching so natural
// language phrasing ("how do I deploy this") still reaches its keywords
// without loosening the AND guarantee for meaningful terms.
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "can": true,
	"do": true, "does": true, "for": true, "how": true, "i": true,
	"in": true, "is": true, "it": true, "me": true, "my": true,
	"of": true, "on": true, "or": true, "should": true, "the": true,
	"this": true, "to": true, "we": true, "what": true, "with": true,
	"you": true, "your": true,
}

// Hub is the in-memory skill index. It loads imported skills from a
// directory, keeps them synced to the repository, matches the current user
// input against name/trigger/description with whitespace-split AND term
// semantics (spec SKL-002), and tracks usage through the repository.
// Access to the in-memory skill cache is thread-safe and protected by sync.RWMutex.
type Hub struct {
	mu           sync.RWMutex
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
// skills, and rebuilds the in-memory index from the repository.
func (h *Hub) LoadDir(ctx context.Context, dir string) error {
	return h.Reload(ctx, dir)
}

// Reload re-scans the skills directory, imports all valid skill files,
// updates the in-memory skills cache and refreshes the skill registry file.
func (h *Hub) Reload(ctx context.Context, dir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

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

	if h.registryPath != "" {
		if err := WriteRegistry(h.registryPath, h.skills); err != nil {
			h.logger.Warn("skills: registry write failed during reload", "path", h.registryPath, "error", err)
		}
	}

	return nil
}

// GetSkill retrieves a specific skill by exact name from the in-memory cache.
// Returns (skill, true) if found, or (nil, false) if not found.
func (h *Hub) GetSkill(name string) (*core.Skill, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, s := range h.skills {
		if s.Name == name {
			cp := s
			return &cp, true
		}
	}
	return nil, false
}

// Match returns up to limit skills whose combined name, trigger, and
// description contain every whitespace-separated meaningful term of the
// input (common stop words are dropped first), case-insensitively. A
// non-positive limit falls back to DefaultMatchLimit.
func (h *Hub) Match(input string, limit int) []core.Skill {
	if limit <= 0 {
		limit = DefaultMatchLimit
	}

	var terms []string
	for _, w := range strings.Fields(strings.ToLower(input)) {
		if !stopWords[w] {
			terms = append(terms, w)
		}
	}
	if len(terms) == 0 {
		return nil
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

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
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, s := range h.skills {
		if s.Name == skill.Name {
			h.skills[i] = skill
			return
		}
	}
	h.skills = append(h.skills, skill)
}

// Skills returns the indexed skills in repository order (last_used DESC,
// then name). Callers receive a safe copy.
func (h *Hub) Skills() []core.Skill {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]core.Skill, len(h.skills))
	copy(out, h.skills)
	return out
}
