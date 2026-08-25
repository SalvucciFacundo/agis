package persona

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// evolutionLimit caps how many user-model rows feed the layer.
const evolutionLimit = 5

// userKeyPrefix is trimmed from row keys for readability inside the layer.
const userKeyPrefix = "user/"

// Evolution is the derived persona layer (spec PER-004): a guidance block
// assembled from the top user-model rows by confidence. It is computed state —
// freezing hides the layer, resetting clears the underlying rows, and neither
// ever touches SOUL.md.
type Evolution struct {
	repo   core.Repository
	logger *slog.Logger
	frozen bool
}

// NewEvolution returns an active (unfrozen) Evolution backed by repo.
func NewEvolution(repo core.Repository, logger *slog.Logger) *Evolution {
	if logger == nil {
		logger = slog.Default()
	}
	return &Evolution{repo: repo, logger: logger}
}

// Freeze excludes the layer from prompts for the rest of the session.
func (e *Evolution) Freeze() { e.frozen = true }

// Frozen reports whether the layer is currently excluded.
func (e *Evolution) Frozen() bool { return e.frozen }

// Layer assembles the guidance block from up to five user-model rows ordered
// by confidence. It returns empty text when frozen or when nothing has been
// learned yet.
func (e *Evolution) Layer(ctx context.Context) string {
	if e.frozen {
		return ""
	}

	rows, err := e.repo.UserModelRows(ctx, evolutionLimit)
	if err != nil {
		e.logger.Warn("persona: reading user model failed; skipping evolution layer", "error", err)
		return ""
	}
	if len(rows) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("How to work with this user (learned so far):\n")
	for _, r := range rows {
		key := strings.TrimPrefix(r.Key, userKeyPrefix)
		fmt.Fprintf(&b, "- %s: %s\n", key, r.Value)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Reset clears every user-model row. The rows are derived data — rebuildable
// from observations via AggregateUserModel — so this returns the evolution
// layer to its seed state without touching long-term memory.
func (e *Evolution) Reset(ctx context.Context) error {
	if err := e.repo.ClearUserModel(ctx); err != nil {
		return fmt.Errorf("resetting persona evolution: %w", err)
	}
	return nil
}

// Status is what /persona status renders.
type Status struct {
	Frozen bool
	Rows   int
	Active bool // an unfrozen evolution with at least one learned row
}

// Status reports the current evolution mode and how many rows it draws on.
func (e *Evolution) Status(ctx context.Context) (Status, error) {
	rows := 0
	if !e.frozen {
		all, err := e.repo.UserModelRows(ctx, 0)
		if err != nil {
			return Status{}, fmt.Errorf("reading user model status: %w", err)
		}
		rows = len(all)
	}
	return Status{Frozen: e.frozen, Rows: rows, Active: !e.frozen && rows > 0}, nil
}
