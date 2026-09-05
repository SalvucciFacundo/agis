package updater

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// DefaultBackupTargets lists relative files and directories within $AGIS_HOME to include in backups.
var DefaultBackupTargets = []string{
	"agis.db",
	"config.yaml",
	"policy.yaml",
	"SOUL.md",
	"skills",
	"plugins",
}

// CreateBackup archives critical state files from agisHome into a compressed tarball
// located at destDir. If destDir is empty, it defaults to filepath.Join(agisHome, "backups").
// It returns the full path of the created backup file.
func CreateBackup(agisHome string, destDir string) (string, error) {
	if agisHome == "" {
		return "", errors.New("agis home directory cannot be empty")
	}

	if destDir == "" {
		destDir = filepath.Join(agisHome, "backups")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("creating backup destination directory %q: %w", destDir, err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupFileName := fmt.Sprintf("agis-backup-%s.tar.gz", timestamp)
	backupFilePath := filepath.Join(destDir, backupFileName)

	var filesToArchive []string
	for _, target := range DefaultBackupTargets {
		fullPath := filepath.Join(agisHome, target)
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("checking backup target %q: %w", target, err)
		}

		if info.IsDir() {
			err = filepath.Walk(fullPath, func(path string, walkInfo os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !walkInfo.IsDir() {
					filesToArchive = append(filesToArchive, path)
				}
				return nil
			})
			if err != nil {
				return "", fmt.Errorf("scanning directory %q: %w", target, err)
			}
		} else {
			filesToArchive = append(filesToArchive, fullPath)
		}
	}

	if len(filesToArchive) == 0 {
		return "", fmt.Errorf("no files found to backup in %s", agisHome)
	}

	tarFile, err := os.OpenFile(backupFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating backup archive file %q: %w", backupFilePath, err)
	}
	defer tarFile.Close()

	gw := gzip.NewWriter(tarFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, filePath := range filesToArchive {
		relPath, err := filepath.Rel(agisHome, filePath)
		if err != nil {
			return "", fmt.Errorf("determining relative path for %q: %w", filePath, err)
		}
		// Standardize separators to forward slash in tar header
		relPath = filepath.ToSlash(relPath)

		if err := addFileToTar(tw, filePath, relPath); err != nil {
			return "", fmt.Errorf("archiving file %q: %w", filePath, err)
		}
	}

	return backupFilePath, nil
}

func addFileToTar(tw *tar.Writer, filePath, relPath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = relPath

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	if _, err := io.Copy(tw, f); err != nil {
		return err
	}

	return nil
}
