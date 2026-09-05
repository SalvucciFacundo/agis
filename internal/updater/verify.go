// Package updater provides backup, release inspection, checksum verification,
// and in-place self-updating for the AGIS executable.
package updater

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrChecksumMismatch is returned when the downloaded asset SHA-256 does not match the checksums manifest.
	ErrChecksumMismatch = errors.New("checksum mismatch")
	// ErrChecksumNotFound is returned when the asset is not listed in the checksums file.
	ErrChecksumNotFound = errors.New("checksum not found in manifest")
)

// VerifyChecksum parses checksums text and compares the computed SHA-256 hash of assetBytes
// against the expected hash for assetName.
func VerifyChecksum(assetBytes []byte, assetName string, checksumsText string) error {
	baseName := filepath.Base(assetName)
	computedHash := fmt.Sprintf("%x", sha256.Sum256(assetBytes))

	scanner := bufio.NewScanner(strings.NewReader(checksumsText))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		expectedHash := strings.ToLower(fields[0])
		fileName := fields[1]
		fileName = strings.TrimPrefix(fileName, "*") // remove binary mode prefix if present
		fileName = filepath.Base(fileName)

		if strings.EqualFold(fileName, baseName) {
			if !strings.EqualFold(expectedHash, computedHash) {
				return fmt.Errorf("%w: expected %s, got %s for %s", ErrChecksumMismatch, expectedHash, computedHash, baseName)
			}
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	return fmt.Errorf("%w: %s", ErrChecksumNotFound, baseName)
}
