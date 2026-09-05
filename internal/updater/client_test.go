package updater_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SalvucciFacundo/agis/internal/updater"
)

func TestClient_FetchLatestRelease(t *testing.T) {
	mockRelease := updater.Release{
		TagName:     "v0.4.0",
		Name:        "Release v0.4.0",
		PublishedAt: "2026-03-01T12:00:00Z",
		HTMLURL:     "https://github.com/SalvucciFacundo/agis/releases/tag/v0.4.0",
		Assets: []updater.Asset{
			{
				Name:               "agis_linux_amd64.tar.gz",
				Size:               1024,
				BrowserDownloadURL: "https://example.com/agis_linux_amd64.tar.gz",
			},
			{
				Name:               "checksums.txt",
				Size:               256,
				BrowserDownloadURL: "https://example.com/checksums.txt",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/SalvucciFacundo/agis/releases/latest", r.URL.Path)
		assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
		assert.Contains(t, r.Header.Get("User-Agent"), "agis-updater")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	client := updater.NewClient("SalvucciFacundo/agis", updater.WithBaseURL(ts.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rel, err := client.FetchLatestRelease(ctx)
	require.NoError(t, err)
	assert.Equal(t, "v0.4.0", rel.TagName)
	assert.Len(t, rel.Assets, 2)
}

func TestClient_FetchReleaseByTag(t *testing.T) {
	mockRelease := updater.Release{
		TagName: "v0.3.5",
		Name:    "Release v0.3.5",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "v0.3.5") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockRelease)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer ts.Close()

	client := updater.NewClient("SalvucciFacundo/agis", updater.WithBaseURL(ts.URL))
	ctx := context.Background()

	t.Run("existing tag", func(t *testing.T) {
		rel, err := client.FetchReleaseByTag(ctx, "v0.3.5")
		require.NoError(t, err)
		assert.Equal(t, "v0.3.5", rel.TagName)
	})

	t.Run("non-existent tag returns error", func(t *testing.T) {
		_, err := client.FetchReleaseByTag(ctx, "v99.0.0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("empty tag returns error", func(t *testing.T) {
		_, err := client.FetchReleaseByTag(ctx, "")
		require.Error(t, err)
	})
}

func TestClient_DownloadAsset(t *testing.T) {
	payload := []byte("downloaded binary asset content")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "rate-limited") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	client := updater.NewClient("SalvucciFacundo/agis", updater.WithBaseURL(ts.URL))

	t.Run("successful download", func(t *testing.T) {
		asset := &updater.Asset{
			Name:               "agis_linux_amd64.tar.gz",
			BrowserDownloadURL: ts.URL + "/download/agis_linux_amd64.tar.gz",
		}
		data, err := client.DownloadAsset(context.Background(), asset)
		require.NoError(t, err)
		assert.Equal(t, payload, data)
	})

	t.Run("server error on download", func(t *testing.T) {
		asset := &updater.Asset{
			Name:               "agis_linux_amd64.tar.gz",
			BrowserDownloadURL: ts.URL + "/rate-limited",
		}
		_, err := client.DownloadAsset(context.Background(), asset)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTP 403")
	})

	t.Run("context cancellation during download", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		asset := &updater.Asset{
			Name:               "agis_linux_amd64.tar.gz",
			BrowserDownloadURL: ts.URL + "/download/agis_linux_amd64.tar.gz",
		}
		_, err := client.DownloadAsset(ctx, asset)
		require.Error(t, err)
	})

	t.Run("nil asset returns error", func(t *testing.T) {
		_, err := client.DownloadAsset(context.Background(), nil)
		require.Error(t, err)
	})
}

func TestFindAssetForPlatform(t *testing.T) {
	release := &updater.Release{
		Assets: []updater.Asset{
			{Name: "agis_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
			{Name: "agis_linux_arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux-arm64"},
			{Name: "agis_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin-amd64"},
			{Name: "agis_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin-arm64"},
			{Name: "agis_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows-amd64"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
		},
	}

	tests := []struct {
		name          string
		goos          string
		goarch        string
		expectedAsset string
	}{
		{name: "linux amd64", goos: "linux", goarch: "amd64", expectedAsset: "agis_linux_amd64.tar.gz"},
		{name: "linux arm64", goos: "linux", goarch: "arm64", expectedAsset: "agis_linux_arm64.tar.gz"},
		{name: "darwin arm64 (apple silicon)", goos: "darwin", goarch: "arm64", expectedAsset: "agis_darwin_arm64.tar.gz"},
		{name: "darwin amd64 (intel mac)", goos: "darwin", goarch: "amd64", expectedAsset: "agis_darwin_amd64.tar.gz"},
		{name: "windows amd64", goos: "windows", goarch: "amd64", expectedAsset: "agis_windows_amd64.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binAsset, chkAsset, err := updater.FindAssetForPlatform(release, tt.goos, tt.goarch)
			require.NoError(t, err)
			require.NotNil(t, binAsset)
			require.NotNil(t, chkAsset)
			assert.Equal(t, tt.expectedAsset, binAsset.Name)
			assert.Equal(t, "checksums.txt", chkAsset.Name)
		})
	}

	t.Run("nil release returns error", func(t *testing.T) {
		_, _, err := updater.FindAssetForPlatform(nil, "linux", "amd64")
		require.Error(t, err)
	})

	t.Run("unsupported platform returns error", func(t *testing.T) {
		_, _, err := updater.FindAssetForPlatform(release, "plan9", "386")
		require.Error(t, err)
	})

	t.Run("missing checksums asset returns error", func(t *testing.T) {
		relNoChecksums := &updater.Release{
			Assets: []updater.Asset{
				{Name: "agis_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
			},
		}
		_, _, err := updater.FindAssetForPlatform(relNoChecksums, "linux", "amd64")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checksum")
	})
}

func TestExtractBinaryFromAsset(t *testing.T) {
	t.Run("extracts binary from tar.gz", func(t *testing.T) {
		binaryContent := []byte("#!/bin/sh\necho updated\n")
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		hdr := &tar.Header{
			Name: "agis",
			Mode: 0o755,
			Size: int64(len(binaryContent)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(binaryContent)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())

		extracted, err := updater.ExtractBinaryFromAsset("agis_linux_amd64.tar.gz", buf.Bytes())
		require.NoError(t, err)
		assert.Equal(t, binaryContent, extracted)
	})

	t.Run("extracts binary from zip", func(t *testing.T) {
		binaryContent := []byte("binary in zip")
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)

		w, err := zw.Create("agis.exe")
		require.NoError(t, err)
		_, err = w.Write(binaryContent)
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		extracted, err := updater.ExtractBinaryFromAsset("agis_windows_amd64.zip", buf.Bytes())
		require.NoError(t, err)
		assert.Equal(t, binaryContent, extracted)
	})

	t.Run("returns raw bytes if not an archive", func(t *testing.T) {
		rawBinary := []byte("raw-executable-bytes")
		extracted, err := updater.ExtractBinaryFromAsset("agis", rawBinary)
		require.NoError(t, err)
		assert.Equal(t, rawBinary, extracted)
	})

	t.Run("corrupted tar.gz returns error", func(t *testing.T) {
		_, err := updater.ExtractBinaryFromAsset("agis_linux_amd64.tar.gz", []byte("not-a-gzip"))
		require.Error(t, err)
	})

	t.Run("corrupted zip returns error", func(t *testing.T) {
		_, err := updater.ExtractBinaryFromAsset("agis_windows_amd64.zip", []byte("not-a-zip"))
		require.Error(t, err)
	})
}
