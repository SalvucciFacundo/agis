package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
)

// Tool represents a tool exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// Client represents the MCP JSON-RPC 2.0 client interface.
type Client interface {
	// Initialize performs the protocol handshake and sends notifications/initialized.
	Initialize(ctx context.Context) error

	// ListTools queries available tools from the server with pagination.
	ListTools(ctx context.Context) ([]Tool, error)

	// CallTool invokes a tool by name with arguments and formats text response or errors.
	CallTool(ctx context.Context, name string, args any) (string, error)

	// Close terminates the transport and cleans up background workers.
	Close() error
}

// ClientInfo describes AGIS to the MCP server.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities declares supported MCP client features.
type ClientCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability declares tools support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams is sent in the "initialize" request.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ListToolsParams is sent in the "tools/list" request.
type ListToolsParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// ListToolsResult is the expected payload in a "tools/list" response.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolParams is sent in the "tools/call" request.
type CallToolParams struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

// ContentBlock is an individual item within CallToolResult.
type ContentBlock struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// CallToolResult is the payload returned by "tools/call".
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type mcpClient struct {
	tr       transport.Transport
	reqID    atomic.Uint64
	mu       sync.Mutex
	pending  map[string]chan *JSONRPCResponse
	closed   bool
	closedCh chan struct{}
	recvDone chan struct{}
}

// NewClient creates a new MCP client over the provided Transport.
func NewClient(tr transport.Transport) Client {
	c := &mcpClient{
		tr:       tr,
		pending:  make(map[string]chan *JSONRPCResponse),
		closedCh: make(chan struct{}),
		recvDone: make(chan struct{}),
	}
	go c.receiveLoop()
	return c
}

func (c *mcpClient) receiveLoop() {
	defer close(c.recvDone)

	for {
		// Use a detached background context for reading from transport.
		data, err := c.tr.Receive(context.Background())
		if err != nil {
			c.failAllPending(err)
			return
		}

		resp, err := ParseResponse(data)
		if err == nil && resp != nil && resp.ID != "" {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()

			if ok && ch != nil {
				select {
				case ch <- resp:
				default:
				}
			}
		}
	}
}

func (c *mcpClient) failAllPending(cause error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, ch := range c.pending {
		delete(c.pending, id)
		if ch != nil {
			close(ch)
		}
	}
}

func (c *mcpClient) sendRequest(ctx context.Context, method string, params any) (*JSONRPCResponse, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("mcp client is closed")
	}

	id := strconv.FormatUint(c.reqID.Add(1), 10)
	ch := make(chan *JSONRPCResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req, err := NewRequest(id, method, params)
	if err != nil {
		c.removePending(id)
		return nil, fmt.Errorf("creating request: %w", err)
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		c.removePending(id)
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	if err := c.tr.Send(ctx, reqBytes); err != nil {
		c.removePending(id)
		return nil, fmt.Errorf("sending request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	case <-c.closedCh:
		c.removePending(id)
		return nil, errors.New("mcp client closed")
	case resp, ok := <-ch:
		if !ok || resp == nil {
			return nil, errors.New("mcp transport disconnected")
		}
		return resp, nil
	}
}

func (c *mcpClient) removePending(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pending, id)
}

func (c *mcpClient) sendNotification(ctx context.Context, method string, params any) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("mcp client is closed")
	}
	c.mu.Unlock()

	notif, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("creating notification: %w", err)
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshaling notification: %w", err)
	}

	return c.tr.Send(ctx, data)
}

func (c *mcpClient) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCapabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		ClientInfo: ClientInfo{
			Name:    "agis",
			Version: "1.0.0",
		},
	}

	resp, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("mcp initialize failed: %w", err)
	}

	if resp.Error != nil {
		return resp.Error
	}

	// Send notifications/initialized as required by MCP lifecycle spec
	if err := c.sendNotification(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("sending notifications/initialized: %w", err)
	}

	return nil
}

func (c *mcpClient) ListTools(ctx context.Context) ([]Tool, error) {
	var allTools []Tool
	var cursor string

	for {
		var params any
		if cursor != "" {
			params = ListToolsParams{Cursor: cursor}
		}

		resp, err := c.sendRequest(ctx, "tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("tools/list request failed: %w", err)
		}

		if resp.Error != nil {
			return nil, resp.Error
		}

		var result ListToolsResult
		if len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				return nil, fmt.Errorf("unmarshaling tools/list result: %w", err)
			}
		}

		allTools = append(allTools, result.Tools...)

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	if allTools == nil {
		allTools = []Tool{}
	}

	return allTools, nil
}

func (c *mcpClient) CallTool(ctx context.Context, name string, args any) (string, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("tools/call request failed: %w", err)
	}

	if resp.Error != nil {
		return "", resp.Error
	}

	var result CallToolResult
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return "", fmt.Errorf("unmarshaling tools/call result: %w", err)
		}
	}

	var textParts []string
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			textParts = append(textParts, block.Text)
		}
	}

	combined := strings.Join(textParts, "\n")

	if result.IsError {
		if combined == "" {
			return "", errors.New("tool execution failed with isError=true")
		}
		return "", fmt.Errorf("tool execution error: %s", combined)
	}

	return combined, nil
}

func (c *mcpClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.closedCh)
	c.mu.Unlock()

	err := c.tr.Close()
	<-c.recvDone
	return err
}
