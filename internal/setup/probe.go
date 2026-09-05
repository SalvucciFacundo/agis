package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultProbeTimeout = 5 * time.Second

	defaultOllamaBaseURL     = "http://localhost:11434"
	defaultOpenAIBaseURL     = "https://api.openai.com"
	defaultOpenRouterBaseURL = "https://openrouter.ai/api"
	defaultAnthropicBaseURL  = "https://api.anthropic.com"
)

// ProbeConnectivity tests the network reachability and authentication credentials for the given LLM provider.
// It executes with a bounded timeout of 5 seconds.
func ProbeConnectivity(ctx context.Context, provider, baseURL, apiKey string) error {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = "ollama"
	}

	// Ensure bounded timeout
	probeCtx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
	defer cancel()

	url := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	client := &http.Client{
		Timeout: defaultProbeTimeout,
	}

	switch p {
	case "ollama":
		if url == "" {
			url = defaultOllamaBaseURL
		}
		endpoint := url + "/api/tags"
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("ollama connectivity probe failed (%s): %w", endpoint, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("ollama endpoint returned HTTP %d", resp.StatusCode)
		}
		return nil

	case "openai":
		if url == "" {
			url = defaultOpenAIBaseURL
		}
		endpoint := url + "/v1/models"
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("openai connectivity probe failed (%s): %w", endpoint, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("unauthorized credentials (HTTP %d): invalid API key", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("openai endpoint returned HTTP %d", resp.StatusCode)
		}
		return nil

	case "openrouter":
		if url == "" {
			url = defaultOpenRouterBaseURL
		}
		endpoint := url + "/v1/models"
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("openrouter connectivity probe failed (%s): %w", endpoint, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("unauthorized credentials (HTTP %d): invalid API key", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("openrouter endpoint returned HTTP %d", resp.StatusCode)
		}
		return nil

	case "anthropic":
		if url == "" {
			url = defaultAnthropicBaseURL
		}
		endpoint := url + "/v1/models"
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		if apiKey != "" {
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("anthropic connectivity probe failed (%s): %w", endpoint, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("unauthorized credentials (HTTP %d): invalid API key", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("anthropic endpoint returned HTTP %d", resp.StatusCode)
		}
		return nil

	default:
		return fmt.Errorf("unsupported provider %q for connectivity probe", provider)
	}
}
