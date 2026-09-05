package updater_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/updater"
)

func TestApplyBinary(t *testing.T) {
	t.Run("successfully replaces binary atomically", func(t *testing.T) {
		tempDir := t.TempDir()
		targetBinary := filepath.Join(tempDir, "agis")

		// Create original binary
		origContent := []byte("original binary content")
		require.NoError(t, os.WriteFile(targetBinary, origContent, 0o755))

		// Apply new binary
		newContent := []byte("new updated binary content")
		err := updater.ApplyBinary(newContent, targetBinary)
		require.NoError(t, err)

		// Verify target binary now has new content
		data, err := os.ReadFile(targetBinary)
		require.NoError(t, err)
		assert.Equal(t, newContent, data)

		// Verify permissions are executable (on unix)
		info, err := os.Stat(targetBinary)
		require.NoError(t, err)
		assert.True(t, info.Mode()&0o111 != 0, "binary must be executable")
	})

	t.Run("creates binary if target does not exist yet", func(t *testing.T) {
		tempDir := t.TempDir()
		targetBinary := filepath.Join(tempDir, "new_agis")

		newContent := []byte("brand new binary content")
		err := updater.ApplyBinary(newContent, targetBinary)
		require.NoError(t, err)

		data, err := os.ReadFile(targetBinary)
		require.NoError(t, err)
		assert.Equal(t, newContent, data)
	})

	t.Run("fails when target directory is blocked by a file", func(t *testing.T) {
		tempDir := t.TempDir()
		fileAsDir := filepath.Join(tempDir, "blocker_file")
		require.NoError(t, os.WriteFile(fileAsDir, []byte("blocker"), 0o600))

		invalidTarget := filepath.Join(fileAsDir, "agis")

		err := updater.ApplyBinary([]byte("content"), invalidTarget)
		require.Error(t, err)

		// Check temp dir contains no leftover .tmp files
		entries, readErr := os.ReadDir(tempDir)
		require.NoError(t, readErr)
		for _, e := range entries {
			assert.NotContains(t, e.Name(), ".tmp")
		}
	})

	t.Run("fails with empty binary or empty target path", func(t *testing.T) {
		assert.Error(t, updater.ApplyBinary(nil, "/path/to/agis"))
		assert.Error(t, updater.ApplyBinary([]byte("content"), ""))
	})
}
