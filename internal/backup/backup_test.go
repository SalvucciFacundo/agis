package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/backup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestProfile(t *testing.T, baseHome, profileName string) string {
	t.Helper()
	var profDir string
	if profileName == "default" || profileName == "" {
		profDir = baseHome
	} else {
		profDir = filepath.Join(baseHome, "profiles", profileName)
	}
	require.NoError(t, os.MkdirAll(profDir, 0700))

	// Write config.yaml
	require.NoError(t, os.WriteFile(filepath.Join(profDir, "config.yaml"), []byte("llm:\n  provider: test\n"), 0600))
	// Write agis.db & agis.db-wal
	require.NoError(t, os.WriteFile(filepath.Join(profDir, "agis.db"), []byte("sqlite db data header"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(profDir, "agis.db-wal"), []byte("wal frame data"), 0600))
	// Write SOUL.md
	require.NoError(t, os.WriteFile(filepath.Join(profDir, "SOUL.md"), []byte("# Senior Architect"), 0644))
	// Write policy.yaml
	require.NoError(t, os.WriteFile(filepath.Join(profDir, "policy.yaml"), []byte("rules: []\n"), 0600))
	// Write skill
	skillDir := filepath.Join(profDir, "skills", "sample")
	require.NoError(t, os.MkdirAll(skillDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill instructions"), 0644))

	return profDir
}

func TestBackup_CreateAndInspect(t *testing.T) {
	baseHome := t.TempDir()
	createTestProfile(t, baseHome, "dev")

	var buf bytes.Buffer
	manifest, err := backup.Create("dev", &buf, backup.CreateOptions{
		BaseHome:    baseHome,
		AGISVersion: "v1.4.0",
	})
	require.NoError(t, err)
	require.NotNil(t, manifest)

	assert.Equal(t, "dev", manifest.SourceProfile)
	assert.Equal(t, "v1.4.0", manifest.AGISVersion)
	assert.Equal(t, 1, manifest.Version)
	assert.Contains(t, manifest.Files, "config.yaml")
	assert.Contains(t, manifest.Files, "agis.db")
	assert.Contains(t, manifest.Files, "agis.db-wal")
	assert.Contains(t, manifest.Files, "SOUL.md")
	assert.Contains(t, manifest.Files, "policy.yaml")
	assert.Contains(t, manifest.Files, filepath.Join("skills", "sample", "SKILL.md"))

	// Test Inspect
	inspectManifest, err := backup.Inspect(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, manifest.SourceProfile, inspectManifest.SourceProfile)
	assert.Equal(t, len(manifest.Files), len(inspectManifest.Files))
}

func TestBackup_Restore_Success(t *testing.T) {
	baseHome := t.TempDir()
	createTestProfile(t, baseHome, "source-prof")

	var buf bytes.Buffer
	_, err := backup.Create("source-prof", &buf, backup.CreateOptions{
		BaseHome: baseHome,
	})
	require.NoError(t, err)

	// Restore into target-prof
	destHome := t.TempDir()
	report, err := backup.Restore(bytes.NewReader(buf.Bytes()), "target-prof", backup.RestoreOptions{
		BaseHome: destHome,
	})
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "target-prof", report.ProfileName)

	targetDir := filepath.Join(destHome, "profiles", "target-prof")
	cfgData, err := os.ReadFile(filepath.Join(targetDir, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "llm:\n  provider: test\n", string(cfgData))

	dbData, err := os.ReadFile(filepath.Join(targetDir, "agis.db"))
	require.NoError(t, err)
	assert.Equal(t, "sqlite db data header", string(dbData))

	walData, err := os.ReadFile(filepath.Join(targetDir, "agis.db-wal"))
	require.NoError(t, err)
	assert.Equal(t, "wal frame data", string(walData))

	skillData, err := os.ReadFile(filepath.Join(targetDir, "skills", "sample", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "skill instructions", string(skillData))
}

func TestBackup_Restore_DefaultProfileFallback(t *testing.T) {
	baseHome := t.TempDir()
	createTestProfile(t, baseHome, "my-agent")

	var buf bytes.Buffer
	_, err := backup.Create("my-agent", &buf, backup.CreateOptions{
		BaseHome: baseHome,
	})
	require.NoError(t, err)

	destHome := t.TempDir()
	// Target profile name omitted -> should fallback to source profile name "my-agent"
	report, err := backup.Restore(bytes.NewReader(buf.Bytes()), "", backup.RestoreOptions{
		BaseHome: destHome,
	})
	require.NoError(t, err)
	assert.Equal(t, "my-agent", report.ProfileName)
	assert.FileExists(t, filepath.Join(destHome, "profiles", "my-agent", "config.yaml"))
}

func TestBackup_Restore_OverwriteProtection(t *testing.T) {
	baseHome := t.TempDir()
	createTestProfile(t, baseHome, "existing")

	var buf bytes.Buffer
	_, err := backup.Create("existing", &buf, backup.CreateOptions{
		BaseHome: baseHome,
	})
	require.NoError(t, err)

	// Attempt restore over existing without Force -> should fail
	_, err = backup.Restore(bytes.NewReader(buf.Bytes()), "existing", backup.RestoreOptions{
		BaseHome: baseHome,
		Force:    false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Attempt restore with Force -> should succeed
	report, err := backup.Restore(bytes.NewReader(buf.Bytes()), "existing", backup.RestoreOptions{
		BaseHome: baseHome,
		Force:    true,
	})
	require.NoError(t, err)
	assert.Equal(t, "existing", report.ProfileName)
}

func TestBackup_Restore_PathTraversalRejected(t *testing.T) {
	// Craft a malicious tar.gz archive containing ../evil.txt
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write valid manifest pointing to evil file
	manifest := backup.Manifest{
		Version:       1,
		SourceProfile: "evil",
		Files: map[string]backup.FileInfo{
			"../evil.txt": {Size: 4, SHA256: "dummy"},
		},
	}
	mData, _ := json.Marshal(manifest)
	_ = tw.WriteHeader(&tar.Header{
		Name: backup.ManifestFileName,
		Size: int64(len(mData)),
		Mode: 0600,
	})
	_, _ = tw.Write(mData)

	_ = tw.WriteHeader(&tar.Header{
		Name: "../evil.txt",
		Size: 4,
		Mode: 0600,
	})
	_, _ = tw.Write([]byte("evil"))

	_ = tw.Close()
	_ = gw.Close()

	destHome := t.TempDir()
	_, err := backup.Restore(bytes.NewReader(buf.Bytes()), "test", backup.RestoreOptions{
		BaseHome: destHome,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal rejected")
}

func TestBackup_Restore_ChecksumMismatchRollback(t *testing.T) {
	baseHome := t.TempDir()
	createTestProfile(t, baseHome, "tamper")

	var buf bytes.Buffer
	_, err := backup.Create("tamper", &buf, backup.CreateOptions{
		BaseHome: baseHome,
	})
	require.NoError(t, err)

	// Corrupt a byte in the tar payload (after manifest)
	raw := buf.Bytes()
	// Tamper bytes in middle
	tampered := make([]byte, len(raw))
	copy(tampered, raw)
	tampered[len(tampered)-50] ^= 0xFF

	destHome := t.TempDir()
	_, err = backup.Restore(bytes.NewReader(tampered), "tamper-dest", backup.RestoreOptions{
		BaseHome: destHome,
	})
	require.Error(t, err)
}
