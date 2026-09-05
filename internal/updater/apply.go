package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ApplyBinary writes newBinaryData to a temporary file and atomically replaces targetPath.
// If targetPath is locked (common on Windows), it renames the current binary to .old before moving.
func ApplyBinary(newBinaryData []byte, targetPath string) error {
	if len(newBinaryData) == 0 {
		return errors.New("cannot apply empty binary data")
	}
	if targetPath == "" {
		return errors.New("target executable path cannot be empty")
	}

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %q: %w", targetDir, err)
	}

	tempFile, err := os.CreateTemp(targetDir, ".agis-update-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file in %q: %w", targetDir, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(newBinaryData); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("writing new binary to temporary file: %w", err)
	}

	// Set executable permissions
	if err := tempFile.Chmod(0o755); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("setting executable permissions on %q: %w", tempPath, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temporary binary file: %w", err)
	}

	return replaceExecutable(tempPath, targetPath)
}

func replaceExecutable(tempPath, targetPath string) error {
	// Try standard rename first
	if err := os.Rename(tempPath, targetPath); err == nil {
		return nil
	}

	// If on Windows or rename failed, attempt rename-old dance
	if runtime.GOOS == "windows" || os.PathSeparator == '\\' {
		oldPath := targetPath + ".old"
		_ = os.Remove(oldPath) // remove previous backup if exists

		if err := os.Rename(targetPath, oldPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("renaming existing executable to %q: %w", oldPath, err)
		}

		if err := os.Rename(tempPath, targetPath); err != nil {
			// Try to restore old if replacement failed
			_ = os.Rename(oldPath, targetPath)
			return fmt.Errorf("moving new executable to %q: %w", targetPath, err)
		}
		return nil
	}

	return fmt.Errorf("replacing binary at %q: %w", targetPath, os.Rename(tempPath, targetPath))
}
