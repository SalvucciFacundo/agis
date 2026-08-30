package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SSEConfig defines configuration for establishing an SSE MCP connection.
type SSEConfig struct {
	URL        string
	HTTPClient *http.Client
	Headers    map[string]string
	Logger     *slog.Logger
}

// SSETransport manages communication with an MCP server over Server-Sent Events (SSE) and HTTP POST.
type SSETransport struct {
	baseURL    *url.URL
	httpClient *http.Client
	headers    map[string]string
	logger     *slog.Logger

	postEndpoint      string
	postEndpointReady chan struct{}
	endpointOnce      sync.Once

	incoming chan readResult
	closed   chan struct{}

	streamCtx    context.Context
	streamCancel context.CancelFunc
	closeOnce    sync.Once
	closeErr     error

	wg sync.WaitGroup
	mu sync.RWMutex
}

// NewSSE creates and connects an SSETransport to the remote endpoint.
func NewSSE(cfg SSEConfig) (*SSETransport, error) {
	if cfg.URL == "" {
		return nil, errors.New("mcp sse transport: url is required")
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing mcp sse url %q: %w", cfg.URL, err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 0, // Streaming connection should not have request timeout on client
		}
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &SSETransport{
		baseURL:           parsedURL,
		httpClient:        client,
		headers:           cfg.Headers,
		logger:            logger,
		postEndpointReady: make(chan struct{}),
		incoming:          make(chan readResult, 16),
		closed:            make(chan struct{}),
		streamCtx:         ctx,
		streamCancel:      cancel,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating sse request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting to sse endpoint %q: %w", cfg.URL, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("sse connection failed with status code %d", resp.StatusCode)
	}

	t.wg.Add(1)
	go t.readEventStream(resp.Body)

	return t, nil
}

func (t *SSETransport) readEventStream(body io.ReadCloser) {
	defer func() {
		_ = body.Close()
		t.wg.Done()
	}()

	reader := bufio.NewReader(body)
	var currentEvent string
	var currentData strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")

			if line == "" {
				// Event boundary: dispatch accumulated event
				t.dispatchEvent(currentEvent, currentData.String())
				currentEvent = ""
				currentData.Reset()
			} else if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				dataVal := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if currentData.Len() > 0 {
					currentData.WriteString("\n")
				}
				currentData.WriteString(dataVal)
			}
		}

		if err != nil {
			select {
			case t.incoming <- readResult{err: err}:
			case <-t.closed:
			}
			return
		}
	}
}

func (t *SSETransport) dispatchEvent(event string, data string) {
	if data == "" {
		return
	}

	switch event {
	case "endpoint":
		t.mu.Lock()
		endpointURI := strings.TrimSpace(data)
		if strings.HasPrefix(endpointURI, "http://") || strings.HasPrefix(endpointURI, "https://") {
			t.postEndpoint = endpointURI
		} else {
			rel, err := url.Parse(endpointURI)
			if err == nil {
				t.postEndpoint = t.baseURL.ResolveReference(rel).String()
			} else {
				t.postEndpoint = endpointURI
			}
		}
		t.mu.Unlock()

		t.endpointOnce.Do(func() {
			close(t.postEndpointReady)
		})

	case "message", "":
		dataBytes := []byte(data)
		select {
		case t.incoming <- readResult{data: dataBytes}:
		case <-t.closed:
		}
	default:
		t.logger.Debug("mcp sse ignored event", "event", event, "data", data)
	}
}

// Send posts a JSON-RPC message payload to the discovered session endpoint.
func (t *SSETransport) Send(ctx context.Context, msg []byte) error {
	select {
	case <-t.closed:
		return errors.New("mcp sse transport: closed")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Ensure endpoint is discovered before posting
	select {
	case <-t.postEndpointReady:
	case <-t.closed:
		return errors.New("mcp sse transport: closed")
	case <-ctx.Done():
		return ctx.Err()
	}

	t.mu.RLock()
	endpoint := t.postEndpoint
	t.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("creating post request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	postClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := postClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting mcp message to %q: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("posting mcp message returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// Receive reads the next message payload received over the SSE stream.
func (t *SSETransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-t.closed:
		return nil, errors.New("mcp sse transport: closed")
	case res, ok := <-t.incoming:
		if !ok {
			return nil, io.EOF
		}
		if res.err != nil {
			return nil, res.err
		}
		return res.data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close disconnects the SSE stream and releases resources.
func (t *SSETransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closed)
		t.streamCancel()
		t.wg.Wait()
	})
	return t.closeErr
}
