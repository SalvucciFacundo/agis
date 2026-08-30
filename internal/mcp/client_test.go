package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// mockTransport simulates a bidirectional Transport in-memory for testing.
type mockTransport struct {
	mu       sync.Mutex
	closed   bool
	closedCh chan struct{}
	incoming chan []byte
	outgoing chan []byte
	handler  func(req []byte) ([]byte, error)
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		closedCh: make(chan struct{}),
		incoming: make(chan []byte, 50),
		outgoing: make(chan []byte, 50),
	}
}

func (m *mockTransport) Send(ctx context.Context, msg []byte) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New("mock transport closed")
	}
	handler := m.handler
	m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.closedCh:
		return errors.New("mock transport closed")
	case m.outgoing <- msg:
	}

	if handler != nil {
		resp, err := handler(msg)
		if err == nil && resp != nil {
			m.mu.Lock()
			if !m.closed {
				m.incoming <- resp
			}
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *mockTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.closedCh:
		return nil, errors.New("mock transport closed")
	case msg, ok := <-m.incoming:
		if !ok {
			return nil, errors.New("mock transport closed")
		}
		return msg, nil
	}
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.closedCh)
	}
	return nil
}

func (m *mockTransport) injectIncoming(msg []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.incoming <- msg
	}
}

func TestClient_Initialize_Handshake(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	var initializedNotificationReceived atomic.Bool

	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				resp := mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"test-server","version":"1.0.0"}}`),
				}
				data, _ := json.Marshal(resp)
				return data, nil
			}
		}
		var notifObj mcp.JSONRPCNotification
		if err := json.Unmarshal(req, &notifObj); err == nil {
			if notifObj.Method == "notifications/initialized" {
				initializedNotificationReceived.Store(true)
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Initialize(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return initializedNotificationReceived.Load()
	}, 1*time.Second, 10*time.Millisecond, "expected notifications/initialized notification")
}

func TestClient_ListTools_SinglePage(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
				})
			}
			if reqObj.Method == "tools/list" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result: json.RawMessage(`{
						"tools": [
							{
								"name": "echo",
								"description": "Echoes input text",
								"inputSchema": {"type": "object", "properties": {"message": {"type": "string"}}}
							},
							{
								"name": "calc",
								"description": "Calculates arithmetic",
								"inputSchema": {"type": "object"}
							}
						]
					}`),
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, client.Initialize(ctx))

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "echo", tools[0].Name)
	assert.Equal(t, "Echoes input text", tools[0].Description)
	assert.NotEmpty(t, tools[0].InputSchema)
	assert.Equal(t, "calc", tools[1].Name)
}

func TestClient_ListTools_Paginated(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{}`),
				})
			}
			if reqObj.Method == "tools/list" {
				paramsMap, _ := reqObj.Params.(map[string]any)
				cursor, _ := paramsMap["cursor"].(string)

				if cursor == "" {
					return json.Marshal(mcp.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      reqObj.ID,
						Result: json.RawMessage(`{
							"tools": [{"name": "tool_1", "description": "First tool"}],
							"nextCursor": "page-2"
						}`),
					})
				} else if cursor == "page-2" {
					return json.Marshal(mcp.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      reqObj.ID,
						Result: json.RawMessage(`{
							"tools": [{"name": "tool_2", "description": "Second tool"}],
							"nextCursor": ""
						}`),
					})
				}
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, client.Initialize(ctx))

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "tool_1", tools[0].Name)
	assert.Equal(t, "tool_2", tools[1].Name)
}

func TestClient_CallTool_Success(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{}`),
				})
			}
			if reqObj.Method == "tools/call" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result: json.RawMessage(`{
						"content": [
							{"type": "text", "text": "Line 1: Success"},
							{"type": "text", "text": "Line 2: Detail"}
						],
						"isError": false
					}`),
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, client.Initialize(ctx))

	out, err := client.CallTool(ctx, "sample_tool", map[string]any{"arg": "val"})
	require.NoError(t, err)
	assert.Equal(t, "Line 1: Success\nLine 2: Detail", out)
}

func TestClient_CallTool_ErrorResult(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{}`),
				})
			}
			if reqObj.Method == "tools/call" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result: json.RawMessage(`{
						"content": [
							{"type": "text", "text": "file not found"}
						],
						"isError": true
					}`),
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, client.Initialize(ctx))

	out, err := client.CallTool(ctx, "cat_tool", map[string]any{"path": "/missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
	assert.Empty(t, out)
}

func TestClient_CallTool_JSONRPCError(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "tools/call" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Error: &mcp.JSONRPCError{
						Code:    mcp.CodeMethodNotFound,
						Message: "Method not found",
					},
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.CallTool(ctx, "missing_tool", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Method not found")
}

func TestClient_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	// Handler does not respond to tools/call to simulate hang/timeout
	tr.handler = func(req []byte) ([]byte, error) {
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "slow_tool", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestClient_ListTools_Empty(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{}`),
				})
			}
			if reqObj.Method == "tools/list" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Result:  json.RawMessage(`{"tools": []}`),
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, client.Initialize(ctx))

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)
	assert.Empty(t, tools)
	assert.NotNil(t, tools)
}

func TestClient_Initialize_ErrorResponse(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	tr.handler = func(req []byte) ([]byte, error) {
		var reqObj mcp.JSONRPCRequest
		if err := json.Unmarshal(req, &reqObj); err == nil {
			if reqObj.Method == "initialize" {
				return json.Marshal(mcp.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      reqObj.ID,
					Error: &mcp.JSONRPCError{
						Code:    mcp.CodeInternalError,
						Message: "Protocol version unsupported",
					},
				})
			}
		}
		return nil, nil
	}

	client := mcp.NewClient(tr)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Initialize(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Protocol version unsupported")
}

func TestClient_Close_PendingRequestsFailed(t *testing.T) {
	defer goleak.VerifyNone(t)

	tr := newMockTransport()
	// Hang on tools/call
	tr.handler = func(req []byte) ([]byte, error) {
		return nil, nil
	}

	client := mcp.NewClient(tr)

	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := client.CallTool(ctx, "hang", nil)
		errCh <- err
	}()

	// Give goroutine a moment to send and register pending request
	time.Sleep(30 * time.Millisecond)

	// Close client
	err := client.Close()
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("pending request did not unblock on Close")
	}
}

