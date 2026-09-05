package updater_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/updater"
)

func TestVerifyChecksum(t *testing.T) {
	data := []byte("binary content for testing agis updater")
	hash := sha256.Sum256(data)
	hashHex := fmt.Sprintf("%x", hash)

	t.Run("successful verification with standard format", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("%s  agis_linux_amd64.tar.gz\n%s  agis_darwin_arm64.tar.gz\n", hashHex, "1234567890abcdef")
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.NoError(t, err)
	})

	t.Run("successful verification with comments and empty lines", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("# Checksums for AGIS\n\n   \n%s   agis_linux_amd64.tar.gz\n", hashHex)
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.NoError(t, err)
	})

	t.Run("successful verification with star prefix format", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("%s *agis_linux_amd64.tar.gz\n", hashHex)
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.NoError(t, err)
	})

	t.Run("successful verification case insensitive", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("%s  agis_linux_amd64.tar.gz\n", fmt.Sprintf("%X", hash))
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.NoError(t, err)
	})

	t.Run("fails on checksum mismatch", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("%s  agis_linux_amd64.tar.gz\n", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.Error(t, err)
		assert.ErrorIs(t, err, updater.ErrChecksumMismatch)
	})

	t.Run("fails when asset name is not found in checksums", func(t *testing.T) {
		checksumsTxt := fmt.Sprintf("%s  agis_darwin_amd64.tar.gz\n", hashHex)
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", checksumsTxt)
		require.Error(t, err)
		assert.ErrorIs(t, err, updater.ErrChecksumNotFound)
	})

	t.Run("fails with empty checksums content", func(t *testing.T) {
		err := updater.VerifyChecksum(data, "agis_linux_amd64.tar.gz", "")
		require.Error(t, err)
		assert.ErrorIs(t, err, updater.ErrChecksumNotFound)
	})
}
