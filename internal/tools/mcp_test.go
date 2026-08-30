package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/mcp"
)

// mockMCPClient implements mcp.Client for testing tools.MCPRunner.
type mockMCPClient struct {
	initErr    error
	listTools  []mcp.Tool
	listErr    error
	lastTool   string
	lastArgs   any
	callResult string
	callErr    error
	closed     bool
}

func (m *mockMCPClient) Initialize(context.Context) error { return m.initErr }
func (m *mockMCPClient) ListTools(context.Context) ([]mcp.Tool, error) {
	return m.listTools, m.listErr
}
func (m *mockMCPClient) CallTool(_ context.Context, name string, args any) (string, error) {
	m.lastTool = name
	m.lastArgs = args
	return m.callResult, m.callErr
}
func (m *mockMCPClient) Close() error {
	m.closed = true
	return nil
}

// mockMCPManager implements mcp.Manager for testing FromMCPManager.
type mockMCPManager struct {
	tools map[string][]mcp.Tool
	calls map[string]struct {
		tool string
		args any
	}
}

func (m *mockMCPManager) Start(context.Context) error { return nil }
func (m *mockMCPManager) Stop() error                 { return nil }
func (m *mockMCPManager) Servers() map[string]mcp.Client {
	return nil
}
func (m *mockMCPManager) ListAllTools() map[string][]mcp.Tool {
	return m.tools
}
func (m *mockMCPManager) CallTool(_ context.Context, serverName, toolName string, args any) (string, error) {
	if m.calls == nil {
		m.calls = make(map[string]struct {
			tool string
			args any
		})
	}
	m.calls[serverName] = struct {
		tool string
		args any
	}{tool: toolName, args: args}
	if serverName == "error-server" {
		return "", errors.New("call failed")
	}
	return "manager result for " + toolName, nil
}

func TestMCPRunner_Interface(t *testing.T) {
	client := &mockMCPClient{callResult: "hello output"}
	tool := mcp.Tool{
		Name:        "echo",
		Description: "Echoes input string",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
	}

	runner := NewMCPRunner("filesystem", tool, client)
	var _ core.ToolRunner = runner

	if got := runner.Backend(); got != "mcp:filesystem" {
		t.Errorf("Backend() = %q, want mcp:filesystem", got)
	}
	if got := runner.Name(); got != "mcp_filesystem_echo" {
		t.Errorf("Name() = %q, want mcp_filesystem_echo", got)
	}
	if got := runner.Description(); !strings.Contains(got, "Echoes input string") {
		t.Errorf("Description() = %q, want description containing tool description", got)
	}
	if got := runner.ToolName(); got != "echo" {
		t.Errorf("ToolName() = %q, want echo", got)
	}
}

func TestMCPRunner_Run_JSONArguments(t *testing.T) {
	client := &mockMCPClient{callResult: "query result 42"}
	tool := mcp.Tool{Name: "query", Description: "SQL Query"}
	runner := NewMCPRunner("sqlite", tool, client)

	ctx := context.Background()
	out, err := runner.Run(ctx, `{"sql": "SELECT 42;"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "query result 42" {
		t.Errorf("Run() out = %q, want query result 42", out)
	}
	if client.lastTool != "query" {
		t.Errorf("lastTool = %q, want query", client.lastTool)
	}

	argsMap, ok := client.lastArgs.(map[string]any)
	if !ok {
		t.Fatalf("lastArgs type = %T, want map[string]any", client.lastArgs)
	}
	if argsMap["sql"] != "SELECT 42;" {
		t.Errorf("args['sql'] = %v, want SELECT 42;", argsMap["sql"])
	}
}

func TestMCPRunner_Run_EmptyAndWhitespaceArgs(t *testing.T) {
	client := &mockMCPClient{callResult: "ok"}
	tool := mcp.Tool{Name: "ping"}
	runner := NewMCPRunner("net", tool, client)

	out, err := runner.Run(context.Background(), "")
	if err != nil {
		t.Fatalf("Run(\"\") error = %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %q, want ok", out)
	}

	out2, err := runner.Run(context.Background(), "   ")
	if err != nil {
		t.Fatalf("Run(\"  \") error = %v", err)
	}
	if out2 != "ok" {
		t.Errorf("out2 = %q, want ok", out2)
	}
}

func TestMCPRunner_Run_InvalidJSON(t *testing.T) {
	client := &mockMCPClient{callResult: "ok"}
	tool := mcp.Tool{Name: "ping"}
	runner := NewMCPRunner("net", tool, client)

	_, err := runner.Run(context.Background(), "{invalid-json")
	if err == nil {
		t.Fatal("Run() expected error on invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "invalid json") && !strings.Contains(err.Error(), "invalid arguments") {
		t.Errorf("error = %v, want mentioning invalid json", err)
	}
}

func TestMCPRunner_Run_ClientError(t *testing.T) {
	client := &mockMCPClient{callErr: errors.New("remote execution failed")}
	tool := mcp.Tool{Name: "fail_tool"}
	runner := NewMCPRunner("srv", tool, client)

	_, err := runner.Run(context.Background(), `{"a": 1}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "remote execution failed") {
		t.Errorf("error = %v, want containing remote execution failed", err)
	}
}

func TestFromMCPManager(t *testing.T) {
	mgr := &mockMCPManager{
		tools: map[string][]mcp.Tool{
			"zeta": {
				{Name: "tool_z", Description: "Tool Z"},
			},
			"alpha": {
				{Name: "tool_b", Description: "Tool B"},
				{Name: "tool_a", Description: "Tool A"},
			},
		},
	}

	runners := FromMCPManager(mgr)
	if len(runners) != 3 {
		t.Fatalf("len(runners) = %d, want 3", len(runners))
	}

	// Verify deterministic sorting: alpha tools first (tool_a, tool_b), then zeta (tool_z)
	if runners[0].Backend() != "mcp:alpha" || runners[0].Name() != "mcp_alpha_tool_a" {
		t.Errorf("runners[0] = %s / %s, want mcp:alpha / mcp_alpha_tool_a", runners[0].Backend(), runners[0].Name())
	}
	if runners[1].Backend() != "mcp:alpha" || runners[1].Name() != "mcp_alpha_tool_b" {
		t.Errorf("runners[1] = %s / %s, want mcp:alpha / mcp_alpha_tool_b", runners[1].Backend(), runners[1].Name())
	}
	if runners[2].Backend() != "mcp:zeta" || runners[2].Name() != "mcp_zeta_tool_z" {
		t.Errorf("runners[2] = %s / %s, want mcp:zeta / mcp_zeta_tool_z", runners[2].Backend(), runners[2].Name())
	}

	// Test executing a runner created from FromMCPManager
	ctx := context.Background()
	out, err := runners[0].Run(ctx, `{"key": "val"}`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out, "manager result for tool_a") {
		t.Errorf("out = %q, want containing manager result for tool_a", out)
	}
}

func TestFromMCPManager_NilOrEmpty(t *testing.T) {
	if runners := FromMCPManager(nil); len(runners) != 0 {
		t.Errorf("expected 0 runners for nil manager, got %d", len(runners))
	}

	mgr := &mockMCPManager{tools: map[string][]mcp.Tool{}}
	if runners := FromMCPManager(mgr); len(runners) != 0 {
		t.Errorf("expected 0 runners for empty manager, got %d", len(runners))
	}
}
