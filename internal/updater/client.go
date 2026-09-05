package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultGitHubAPI = "https://api.github.com"
	defaultRepo      = "SalvucciFacundo/agis"
	userAgent        = "agis-updater/1.0"
)

// Release represents a GitHub release.
type Release struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []Asset `json:"assets"`
}

// Asset represents an attached asset in a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Client manages communication with the GitHub Releases API.
type Client struct {
	repository string
	httpClient *http.Client
	baseURL    string
}

// Option allows configuring the Client.
type Option func(*Client)

// WithHTTPClient configures a custom http.Client.
func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		if c != nil {
			client.httpClient = c
		}
	}
}

// WithBaseURL configures a custom base URL (useful for test servers).
func WithBaseURL(url string) Option {
	return func(client *Client) {
		client.baseURL = strings.TrimSuffix(url, "/")
	}
}

// NewClient creates a new GitHub release Client for the given repository.
func NewClient(repo string, opts ...Option) *Client {
	if repo == "" {
		repo = defaultRepo
	}

	c := &Client{
		repository: repo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultGitHubAPI,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// FetchLatestRelease retrieves metadata for the repository's latest release.
func (c *Client) FetchLatestRelease(ctx context.Context) (*Release, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, c.repository)
	return c.fetchRelease(ctx, endpoint)
}

// FetchReleaseByTag retrieves metadata for a specific release tag.
func (c *Client) FetchReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("release tag cannot be empty")
	}

	endpoint := fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.baseURL, c.repository, tag)
	return c.fetchRelease(ctx, endpoint)
}

func (c *Client) fetchRelease(ctx context.Context, endpoint string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", endpoint, err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching release from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release not found (HTTP 404) at %s", endpoint)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release JSON: %w", err)
	}

	return &rel, nil
}

// DownloadAsset downloads the raw byte payload of the given Asset.
func (c *Client) DownloadAsset(ctx context.Context, asset *Asset) ([]byte, error) {
	if asset == nil || asset.BrowserDownloadURL == "" {
		return nil, errors.New("invalid asset or missing download URL")
	}

	downloadURL := asset.BrowserDownloadURL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request for %s: %w", asset.Name, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading asset %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading asset %s failed with HTTP %d", asset.Name, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading asset body for %s: %w", asset.Name, err)
	}

	return data, nil
}

// FindAssetForPlatform matches the platform binary archive and checksums file in a release
// matching the specified operating system (goos) and architecture (goarch).
func FindAssetForPlatform(release *Release, goos, goarch string) (*Asset, *Asset, error) {
	if release == nil {
		return nil, nil, errors.New("release cannot be nil")
	}

	var binaryAsset *Asset
	var checksumAsset *Asset

	// Normalize platform search tokens
	platformToken := fmt.Sprintf("%s_%s", goos, goarch)
	altPlatformToken := fmt.Sprintf("%s-%s", goos, goarch)

	for i := range release.Assets {
		asset := &release.Assets[i]
		lowerName := strings.ToLower(asset.Name)

		// Check for checksums file
		if lowerName == "checksums.txt" || lowerName == "sha256sums" || strings.Contains(lowerName, "checksums") {
			checksumAsset = asset
			continue
		}

		// Check for platform binary or archive match
		if strings.Contains(lowerName, platformToken) || strings.Contains(lowerName, altPlatformToken) {
			binaryAsset = asset
		}
	}

	if checksumAsset == nil {
		return nil, nil, fmt.Errorf("checksums asset (checksums.txt) not found in release %s", release.TagName)
	}

	if binaryAsset == nil {
		return nil, nil, fmt.Errorf("no compatible asset found for platform %s/%s in release %s", goos, goarch, release.TagName)
	}

	return binaryAsset, checksumAsset, nil
}

// ExtractBinaryFromAsset extracts the executable from an archive (.tar.gz, .tgz, .zip) or returns raw bytes.
func ExtractBinaryFromAsset(assetName string, data []byte) ([]byte, error) {
	lower := strings.ToLower(assetName)

	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("reading gzip archive %s: %w", assetName, err)
		}
		defer gzr.Close()

		tr := tar.NewReader(gzr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("reading tar entry from %s: %w", assetName, err)
			}

			base := filepath.Base(hdr.Name)
			if base == "agis" || base == "agis.exe" {
				extracted, err := io.ReadAll(tr)
				if err != nil {
					return nil, fmt.Errorf("extracting binary %s: %w", hdr.Name, err)
				}
				return extracted, nil
			}
		}
		return nil, fmt.Errorf("binary 'agis' not found in archive %s", assetName)

	case strings.HasSuffix(lower, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("reading zip archive %s: %w", assetName, err)
		}

		for _, file := range zr.File {
			base := filepath.Base(file.Name)
			if base == "agis" || base == "agis.exe" {
				rc, err := file.Open()
				if err != nil {
					return nil, fmt.Errorf("opening zip entry %s: %w", file.Name, err)
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("binary 'agis' not found in zip archive %s", assetName)

	default:
		// Raw binary
		return data, nil
	}
}
