package updater_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/updater"
)

func TestCreateBackup(t *testing.T) {
	t.Run("creates valid tar.gz containing existing files", func(t *testing.T) {
		homeDir := t.TempDir()
		backupDest := filepath.Join(homeDir, "backups")

		// Create mock AGIS state files and directories
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("llm: model: gpt-4"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "agis.db"), []byte("sqlite-database-data"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "SOUL.md"), []byte("# Persona"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "policy.yaml"), []byte("default: ask"), 0o600))

		skillsDir := filepath.Join(homeDir, "skills", "weather")
		require.NoError(t, os.MkdirAll(skillsDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "skill.yaml"), []byte("name: weather"), 0o600))

		pluginsDir := filepath.Join(homeDir, "plugins", "sample")
		require.NoError(t, os.MkdirAll(pluginsDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "plugin.json"), []byte("{\"name\": \"sample\"}"), 0o600))

		archivePath, err := updater.CreateBackup(homeDir, backupDest)
		require.NoError(t, err)
		assert.NotEmpty(t, archivePath)
		assert.FileExists(t, archivePath)

		// Verify contents of tar.gz
		f, err := os.Open(archivePath)
		require.NoError(t, err)
		defer f.Close()

		gzr, err := gzip.NewReader(f)
		require.NoError(t, err)
		defer gzr.Close()

		tr := tar.NewReader(gzr)
		foundFiles := make(map[string]bool)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			foundFiles[hdr.Name] = true
		}

		assert.True(t, foundFiles["config.yaml"])
		assert.True(t, foundFiles["agis.db"])
		assert.True(t, foundFiles["SOUL.md"])
		assert.True(t, foundFiles["policy.yaml"])
		assert.True(t, foundFiles["skills/weather/skill.yaml"])
		assert.True(t, foundFiles["plugins/sample/plugin.json"])
	})

	t.Run("defaults destDir when empty", func(t *testing.T) {
		homeDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("test"), 0o600))

		archivePath, err := updater.CreateBackup(homeDir, "")
		require.NoError(t, err)
		assert.Contains(t, archivePath, filepath.Join(homeDir, "backups"))
		assert.FileExists(t, archivePath)
	})

	t.Run("empty homeDir returns error", func(t *testing.T) {
		_, err := updater.CreateBackup("", "")
		require.Error(t, err)
	})

	t.Run("skips missing optional files without failing", func(t *testing.T) {
		homeDir := t.TempDir()
		backupDest := filepath.Join(homeDir, "backups")

		// Only config.yaml exists
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("test"), 0o600))

		archivePath, err := updater.CreateBackup(homeDir, backupDest)
		require.NoError(t, err)
		assert.FileExists(t, archivePath)
	})

	t.Run("aborts with error when no state files exist", func(t *testing.T) {
		homeDir := t.TempDir()
		backupDest := filepath.Join(homeDir, "backups")

		_, err := updater.CreateBackup(homeDir, backupDest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no files found to backup")
	})

	t.Run("aborts when backup directory cannot be created or written", func(t *testing.T) {
		homeDir := t.TempDir()
		// Create a file where directory should be
		invalidDest := filepath.Join(homeDir, "blocked_dest")
		require.NoError(t, os.WriteFile(invalidDest, []byte("blocker"), 0o400))
		blockedSubdir := filepath.Join(invalidDest, "sub")

		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("test"), 0o600))

		_, err := updater.CreateBackup(homeDir, blockedSubdir)
		require.Error(t, err)
	})
}
