package skills

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// registryHeader is written at the top of every regeneration.
const registryHeader = "# AGIS Skill Registry\n\nAuto-generated; edit skill files, not this index.\n"

// WriteRegistry atomically regenerates the skill-registry Markdown file at
// path (tmp file + rename) listing the given skills in the order provided.
// Callers treat an error as a warning: a failed registry write must never
// break startup or session close (spec SKL-005).
func WriteRegistry(path string, skills []core.Skill) error {
	if path == "" {
		return nil
	}

	var b strings.Builder
	b.WriteString(registryHeader)
	b.WriteString(fmt.Sprintf("Last updated: %s\n\n",
		time.Now().UTC().Format("2006-01-02 15:04:05 MST")))
	b.WriteString("| Name | Trigger | Source | Uses | Description |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, s := range skills {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %s |\n",
			s.Name, s.Trigger, s.Source, s.UsageCount, s.Description))
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("writing registry tmp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming registry into place: %w", err)
	}
	return nil
}

// SyncRegistry regenerates the Hub's registry file from the current index,
// logging failures as warnings.
func (h *Hub) SyncRegistry(path string) {
	if path == "" || h == nil {
		return
	}
	if err := WriteRegistry(path, h.skills); err != nil {
		h.logger.Warn("skills: registry write failed", "path", path, "error", err)
	}
}
