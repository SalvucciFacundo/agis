package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// RestoreOptions customizes the restore operation.
type RestoreOptions struct {
	BaseHome string
	Force    bool
}

// RestoreReport summarizes the outcome of a successful profile restore.
type RestoreReport struct {
	ProfileName string   `json:"profile_name"`
	TargetDir   string   `json:"target_dir"`
	FilesCount  int      `json:"files_count"`
	Restored    []string `json:"restored"`
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Restore unpacks and verifies a .tar.gz profile archive into the target profile.
func Restore(r io.Reader, targetProfile string, opts RestoreOptions) (*RestoreReport, error) {
	baseHome := opts.BaseHome
	if baseHome == "" {
		baseHome = config.BaseHome()
	}

	// 1. Create temporary staging directory
	stagingDir := filepath.Join(baseHome, ".restore-tmp-"+randomSuffix())
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return nil, fmt.Errorf("creating restore staging directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	// 2. Unpack tarball into staging directory with path traversal protection
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading gzip archive: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("reading tar entry: %w", err)
		}

		cleanName := filepath.Clean(filepath.FromSlash(hdr.Name))
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return nil, fmt.Errorf("security: path traversal rejected in archive path %q", hdr.Name)
		}

		destPath := filepath.Join(stagingDir, cleanName)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0700); err != nil {
				return nil, fmt.Errorf("creating directory %s: %w", cleanName, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			parent := filepath.Dir(destPath)
			if err := os.MkdirAll(parent, 0700); err != nil {
				return nil, fmt.Errorf("creating directory for %s: %w", cleanName, err)
			}

			mode := os.FileMode(hdr.Mode)
			if mode == 0 {
				mode = 0600
			}

			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return nil, fmt.Errorf("creating staged file %s: %w", cleanName, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				_ = outFile.Close()
				return nil, fmt.Errorf("extracting staged file %s: %w", cleanName, err)
			}
			_ = outFile.Close()
		}
	}

	// 3. Read and validate manifest
	manifestPath := filepath.Join(stagingDir, ManifestFileName)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest %s not found in archive: %w", ManifestFileName, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parsing archive manifest: %w", err)
	}

	// 4. Resolve destination profile
	finalProfile := strings.TrimSpace(targetProfile)
	if finalProfile == "" {
		finalProfile = manifest.SourceProfile
		if finalProfile == "" {
			finalProfile = "default"
		}
	}

	var targetDir string
	if finalProfile == "default" {
		targetDir = baseHome
	} else {
		if err := config.ValidateProfileName(finalProfile); err != nil {
			return nil, fmt.Errorf("invalid destination profile name %q: %w", finalProfile, err)
		}
		targetDir = filepath.Join(baseHome, "profiles", finalProfile)
	}

	// 5. Overwrite check
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		hasExistingData := false
		for _, check := range []string{"config.yaml", "agis.db"} {
			if _, err := os.Stat(filepath.Join(targetDir, check)); err == nil {
				hasExistingData = true
				break
			}
		}
		if hasExistingData && !opts.Force {
			return nil, fmt.Errorf("target profile %q already exists at %s (use -force to overwrite)", finalProfile, targetDir)
		}
	}

	// 6. Verify checksums of all staged files
	var restoredFiles []string
	for relPath, fInfo := range manifest.Files {
		stagedFile := filepath.Join(stagingDir, filepath.FromSlash(relPath))
		hash, size, _, err := HashFile(stagedFile)
		if err != nil {
			return nil, fmt.Errorf("verifying staged file %s: %w", relPath, err)
		}
		if size != fInfo.Size {
			return nil, fmt.Errorf("integrity violation: size mismatch for %s (got %d, want %d)", relPath, size, fInfo.Size)
		}
		if hash != fInfo.SHA256 {
			return nil, fmt.Errorf("integrity violation: SHA-256 mismatch for %s (got %s, want %s)", relPath, hash, fInfo.SHA256)
		}
		restoredFiles = append(restoredFiles, relPath)
	}

	// 7. Commit from staging into targetDir
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return nil, fmt.Errorf("creating destination directory %s: %w", targetDir, err)
	}

	for _, relPath := range restoredFiles {
		srcFile := filepath.Join(stagingDir, filepath.FromSlash(relPath))
		dstFile := filepath.Join(targetDir, filepath.FromSlash(relPath))

		if err := os.MkdirAll(filepath.Dir(dstFile), 0700); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", relPath, err)
		}

		data, err := os.ReadFile(srcFile)
		if err != nil {
			return nil, fmt.Errorf("reading staged file %s: %w", relPath, err)
		}

		// Apply security file mode
		fileMode := os.FileMode(0644)
		lowerRel := strings.ToLower(relPath)
		if strings.HasSuffix(lowerRel, "config.yaml") ||
			strings.HasSuffix(lowerRel, "policy.yaml") ||
			strings.Contains(lowerRel, ".db") {
			fileMode = 0600
		}

		if err := os.WriteFile(dstFile, data, fileMode); err != nil {
			return nil, fmt.Errorf("writing destination file %s: %w", relPath, err)
		}
	}

	return &RestoreReport{
		ProfileName: finalProfile,
		TargetDir:   targetDir,
		FilesCount:  len(restoredFiles),
		Restored:    restoredFiles,
	}, nil
}
