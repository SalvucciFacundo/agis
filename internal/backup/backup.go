package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/version"
)

// CreateOptions customizes the backup creation process.
type CreateOptions struct {
	BaseHome    string
	AGISVersion string
}

// Create generates a .tar.gz archive of the specified profile and writes it to w.
func Create(profileName string, w io.Writer, opts CreateOptions) (*Manifest, error) {
	baseHome := opts.BaseHome
	if baseHome == "" {
		baseHome = config.BaseHome()
	}

	trimmedProfile := strings.TrimSpace(profileName)
	if trimmedProfile == "" {
		trimmedProfile = config.ActiveProfile()
		if trimmedProfile == "" {
			trimmedProfile = "default"
		}
	}

	var profDir string
	if trimmedProfile == "default" {
		profDir = baseHome
	} else {
		if err := config.ValidateProfileName(trimmedProfile); err != nil {
			return nil, fmt.Errorf("invalid profile name %q: %w", trimmedProfile, err)
		}
		profDir = filepath.Join(baseHome, "profiles", trimmedProfile)
	}

	if info, err := os.Stat(profDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("profile directory %q does not exist or is not a directory", profDir)
	}

	agisVer := opts.AGISVersion
	if agisVer == "" {
		agisVer = version.Version
		if agisVer == "" {
			agisVer = "v1.4.0"
		}
	}

	manifest := &Manifest{
		Version:       ManifestVersion,
		AGISVersion:   agisVer,
		SourceProfile: trimmedProfile,
		CreatedAt:     time.Now().UTC(),
		Files:         make(map[string]FileInfo),
	}

	// 1. Collect and hash candidates
	candidates := []string{
		"config.yaml",
		"agis.db",
		"agis.db-wal",
		"agis.db-shm",
		"SOUL.md",
		"policy.yaml",
	}

	var collectedPaths []string
	for _, rel := range candidates {
		fullPath := filepath.Join(profDir, rel)
		if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
			hash, size, mode, err := HashFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("hashing file %s: %w", rel, err)
			}
			manifest.Files[rel] = FileInfo{
				Size:   size,
				SHA256: hash,
				Mode:   mode,
			}
			collectedPaths = append(collectedPaths, rel)
		}
	}

	// Walk recursive directories: skills/ and plugins/
	dirsToScan := []string{"skills", "plugins"}
	for _, subDir := range dirsToScan {
		dirPath := filepath.Join(profDir, subDir)
		if _, err := os.Stat(dirPath); err == nil {
			err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(profDir, path)
				if err != nil {
					return err
				}
				hash, size, mode, err := HashFile(path)
				if err != nil {
					return fmt.Errorf("hashing file %s: %w", rel, err)
				}
				manifest.Files[rel] = FileInfo{
					Size:   size,
					SHA256: hash,
					Mode:   mode,
				}
				collectedPaths = append(collectedPaths, rel)
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walking %s directory: %w", subDir, err)
			}
		}
	}

	if len(manifest.Files) == 0 {
		return nil, fmt.Errorf("no archivable files found in profile %q (%s)", trimmedProfile, profDir)
	}

	// 2. Stream to tar.gz
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling manifest: %w", err)
	}

	// Write manifest as first entry
	manifestHdr := &tar.Header{
		Name:     ManifestFileName,
		Mode:     0600,
		Size:     int64(len(manifestBytes)),
		ModTime:  manifest.CreatedAt,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(manifestHdr); err != nil {
		return nil, fmt.Errorf("writing manifest tar header: %w", err)
	}
	if _, err := tw.Write(manifestBytes); err != nil {
		return nil, fmt.Errorf("writing manifest content: %w", err)
	}

	// Write each collected file
	for _, rel := range collectedPaths {
		fullPath := filepath.Join(profDir, rel)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading file %s for archive: %w", rel, err)
		}

		// Normalize tar path to forward slashes
		tarName := filepath.ToSlash(rel)

		hdr := &tar.Header{
			Name:     tarName,
			Mode:     int64(manifest.Files[rel].Mode),
			Size:     int64(len(data)),
			ModTime:  manifest.CreatedAt,
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("writing tar header for %s: %w", rel, err)
		}
		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("writing file %s to tar: %w", rel, err)
		}
	}

	return manifest, nil
}

// Inspect reads and parses the Manifest from a .tar.gz archive without unpacking all files.
func Inspect(r io.Reader) (*Manifest, error) {
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
			return nil, fmt.Errorf("reading tar entries: %w", err)
		}

		if hdr.Name == ManifestFileName {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", ManifestFileName, err)
			}
			var manifest Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("parsing %s: %w", ManifestFileName, err)
			}
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("archive is missing required %s", ManifestFileName)
}
