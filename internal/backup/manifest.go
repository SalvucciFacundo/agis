package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

const (
	// ManifestVersion indicates the format version of the archive manifest.
	ManifestVersion = 1

	// ManifestFileName is the root manifest entry inside the archive.
	ManifestFileName = "manifest.json"
)

// FileInfo stores integrity and size metadata for an archived file.
type FileInfo struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// Manifest represents the top-level metadata of an AGIS profile archive.
type Manifest struct {
	Version       int                 `json:"version"`
	AGISVersion   string              `json:"agis_version"`
	SourceProfile string              `json:"source_profile"`
	CreatedAt     time.Time           `json:"created_at"`
	Files         map[string]FileInfo `json:"files"`
}

// HashFile calculates the SHA-256 hex digest, file size, and file mode.
func HashFile(path string) (string, int64, uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", 0, 0, err
	}

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, 0, err
	}

	return hex.EncodeToString(h.Sum(nil)), size, uint32(stat.Mode().Perm()), nil
}
