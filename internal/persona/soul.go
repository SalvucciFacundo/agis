// Package persona implements AGIS's identity system: the durable SOUL.md
// identity seeded in the AGIS home directory, session-scoped personality
// overlays, and a derived evolution layer assembled from curated user-model
// rows. Evolution never rewrites SOUL.md — it is computed state on top of a
// static seed (spec §8, GAIA "seed + evolution" model).
package persona

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/scan"
)

//go:embed SOUL.default.md
var defaultFS embed.FS

// soulFileName is the durable identity file inside the AGIS home directory.
const soulFileName = "SOUL.md"

// soulPerm matches the config file's privacy stance: identities may contain
// personal guidance and are not other-users' business.
const soulPerm = 0o600

// DefaultIdentity returns the embedded fallback identity, scanned.
func DefaultIdentity() string {
	raw, err := defaultFS.ReadFile("SOUL.default.md")
	if err != nil {
		return ""
	}
	clean, _ := scan.Lines(string(raw))
	return clean
}

// LoadSoul resolves the durable identity from soulPath:
//
//  1. Missing file: seed it from the embedded default (mode 0600) and return
//     that text.
//  2. Present but empty or unreadable: fall back to the embedded default
//     without touching the file.
//  3. Present with content: return it verbatim after injection scanning;
//     flagged lines are dropped and logged. A scan that empties the whole
//     file also falls back to the embedded default.
//
// The error return covers only infrastructure failures of seeding; identity
// resolution itself never fails (spec PER-001).
func LoadSoul(soulPath string, logger *slog.Logger) (string, error) {
	if logger == nil {
		logger = slog.Default()
	}

	data, err := os.ReadFile(soulPath)
	switch {
	case err == nil && strings.TrimSpace(string(data)) != "":
		clean, dropped := scan.Lines(string(data))
		if dropped > 0 {
			logger.Warn("persona: dropped injected lines from SOUL.md", "count", dropped)
		}
		if strings.TrimSpace(clean) == "" {
			logger.Warn("persona: SOUL.md emptied by injection scan; using built-in identity")
			return DefaultIdentity(), nil
		}
		return clean, nil

	case os.IsNotExist(err):
		def := DefaultIdentity()
		if mkErr := os.MkdirAll(filepath.Dir(soulPath), 0o700); mkErr != nil {
			return def, fmt.Errorf("creating AGIS home for SOUL.md: %w", mkErr)
		}
		if writeErr := os.WriteFile(soulPath, []byte(def), soulPerm); writeErr != nil {
			logger.Warn("persona: could not seed SOUL.md; using built-in identity", "error", writeErr)
			return def, nil
		}
		logger.Info("persona: seeded SOUL.md", "path", soulPath)
		return def, nil

	default:
		logger.Warn("persona: SOUL.md unreadable; using built-in identity", "error", err)
		return DefaultIdentity(), nil
	}
}

// SoulPath returns the SOUL.md path inside the given AGIS home directory.
func SoulPath(agisHome string) string {
	return filepath.Join(agisHome, soulFileName)
}
