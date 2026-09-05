package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/updater"
	"github.com/SalvucciFacundo/agis/internal/version"
)

func createMockReleaseServer(t *testing.T, releaseVersion string, binaryContent []byte) *httptest.Server {
	t.Helper()

	var archiveBuf bytes.Buffer
	gw := gzip.NewWriter(&archiveBuf)
	tw := tar.NewWriter(gw)

	binaryName := "agis"
	if runtime.GOOS == "windows" {
		binaryName = "agis.exe"
	}
	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0o755,
		Size: int64(len(binaryContent)),
	}
	require.NoError(t, tw.WriteHeader(hdr))
	_, err := tw.Write(binaryContent)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	archiveData := archiveBuf.Bytes()
	assetFileName := fmt.Sprintf("agis_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	hash := sha256.Sum256(archiveData)
	checksumsContent := fmt.Sprintf("%x  %s\n", hash, assetFileName)

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/SalvucciFacundo/agis/releases/latest" || r.URL.Path == fmt.Sprintf("/repos/SalvucciFacundo/agis/releases/tags/%s", releaseVersion):
			rel := updater.Release{
				TagName: releaseVersion,
				Name:    "Release " + releaseVersion,
				Assets: []updater.Asset{
					{
						Name:               assetFileName,
						Size:               int64(len(archiveData)),
						BrowserDownloadURL: ts.URL + "/download/" + assetFileName,
					},
					{
						Name:               "checksums.txt",
						Size:               int64(len(checksumsContent)),
						BrowserDownloadURL: ts.URL + "/download/checksums.txt",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)

		case r.URL.Path == "/download/"+assetFileName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveData)

		case r.URL.Path == "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return ts
}

func TestRunUpdateCLI(t *testing.T) {
	t.Run("help flag prints usage to stdout with exit code 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--help"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "Usage: agis update")
		assert.Empty(t, stderr.String())
	})

	t.Run("short help flag -h prints usage to stdout with exit code 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"-h"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "Usage: agis update")
		assert.Empty(t, stderr.String())
	})

	t.Run("unknown flag returns exit code 2 and writes to stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--invalid-flag"}, &stdout, &stderr)
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr.String(), "flag provided but not defined")
	})

	t.Run("unexpected positional argument returns exit code 2", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"extra-arg"}, &stdout, &stderr)
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr.String(), "unexpected argument")
	})

	t.Run("check flag reports available update when newer", func(t *testing.T) {
		origVersion := version.Version
		version.Version = "v0.3.0"
		defer func() { version.Version = origVersion }()

		ts := createMockReleaseServer(t, "v0.4.0", []byte("binary"))
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--check"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "Update available: v0.3.0 -> v0.4.0")
		assert.Empty(t, stderr.String())
	})

	t.Run("check flag reports already up to date", func(t *testing.T) {
		origVersion := version.Version
		version.Version = "v0.4.0"
		defer func() { version.Version = origVersion }()

		ts := createMockReleaseServer(t, "v0.4.0", []byte("binary"))
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--check"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "agis is already up to date (v0.4.0)")
		assert.Empty(t, stderr.String())
	})

	t.Run("update when already up to date without force returns 0 and message", func(t *testing.T) {
		origVersion := version.Version
		version.Version = "v0.4.0"
		defer func() { version.Version = origVersion }()

		ts := createMockReleaseServer(t, "v0.4.0", []byte("binary"))
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "agis is already up to date (v0.4.0). Use --force to re-install.")
	})

	t.Run("target specific version tag with --version", func(t *testing.T) {
		tempDir := t.TempDir()
		mockExe := filepath.Join(tempDir, "mock_agis")
		require.NoError(t, os.WriteFile(mockExe, []byte("old binary"), 0o755))

		origTargetExe := testTargetExePath
		testTargetExePath = mockExe
		defer func() { testTargetExePath = origTargetExe }()

		newBinaryContent := []byte("version v0.3.5 bytes")
		ts := createMockReleaseServer(t, "v0.3.5", newBinaryContent)
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--version", "v0.3.5", "--force"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "Successfully updated agis to v0.3.5")
	})

	t.Run("full in-place update with backup and force", func(t *testing.T) {
		tempDir := t.TempDir()
		mockExe := filepath.Join(tempDir, "mock_agis")
		require.NoError(t, os.WriteFile(mockExe, []byte("old binary"), 0o755))

		origTargetExe := testTargetExePath
		testTargetExePath = mockExe
		defer func() { testTargetExePath = origTargetExe }()

		// Setup mock AGIS_HOME
		homeDir := filepath.Join(tempDir, "agis_home")
		require.NoError(t, os.MkdirAll(homeDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("llm: model: gpt-4"), 0o600))
		t.Setenv("AGIS_HOME", homeDir)

		newBinaryContent := []byte("newly updated executable binary bytes")
		ts := createMockReleaseServer(t, "v0.4.0", newBinaryContent)
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--backup", "--force"}, &stdout, &stderr)
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout.String(), "Successfully updated agis to v0.4.0")
		assert.Contains(t, stderr.String(), "backup")

		// Verify binary was actually updated
		updatedContent, err := os.ReadFile(mockExe)
		require.NoError(t, err)
		assert.Equal(t, newBinaryContent, updatedContent)
	})

	t.Run("backup failure aborts update with code 1", func(t *testing.T) {
		tempDir := t.TempDir()
		mockExe := filepath.Join(tempDir, "mock_agis")
		require.NoError(t, os.WriteFile(mockExe, []byte("old binary"), 0o755))

		origTargetExe := testTargetExePath
		testTargetExePath = mockExe
		defer func() { testTargetExePath = origTargetExe }()

		// Empty AGIS_HOME so backup fails
		emptyHome := filepath.Join(tempDir, "empty_home")
		require.NoError(t, os.MkdirAll(emptyHome, 0o755))
		t.Setenv("AGIS_HOME", emptyHome)

		ts := createMockReleaseServer(t, "v0.4.0", []byte("new bytes"))
		defer ts.Close()

		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = ts.URL
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--backup", "--force"}, &stdout, &stderr)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr.String(), "backup failed")
	})

	t.Run("network error returns exit code 1", func(t *testing.T) {
		origBaseURL := testGitHubBaseURL
		testGitHubBaseURL = "http://127.0.0.1:54321/unreachable"
		defer func() { testGitHubBaseURL = origBaseURL }()

		var stdout, stderr bytes.Buffer
		code := RunUpdateCLI([]string{"--check"}, &stdout, &stderr)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr.String(), "agis update:")
	})
}
