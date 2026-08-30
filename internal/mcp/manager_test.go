package mcp_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// mockClient implements mcp.Client for unit testing the Manager.
type mockClient struct {
	mu           sync.Mutex
	initCalled   atomic.Bool
	closedCalled atomic.Bool
	tools        []mcp.Tool
	callFunc     func(ctx context.Context, name string, args any) (string, error)
	initErr      error
}

func (m *mockClient) Initialize(ctx context.Context) error {
	m.initCalled.Store(true)
	if m.initErr != nil {
		return m.initErr
	}
	return nil
}

func (m *mockClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tools, nil
}

func (m *mockClient) CallTool(ctx context.Context, name string, args any) (string, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, name, args)
	}
	return fmt.Sprintf("result_of_%s", name), nil
}

func (m *mockClient) Close() error {
	m.closedCalled.Store(true)
	return nil
}

func TestManager_DisabledConfig(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := config.MCPConfig{
		Enabled: false,
		Servers: map[string]config.MCPServerConfig{
			"s1": {Command: "cat"},
		},
	}

	mgr := mcp.NewManager(cfg)
	ctx := context.Background()

	err := mgr.Start(ctx)
	require.NoError(t, err)

	assert.Empty(t, mgr.Servers())
	assert.Empty(t, mgr.ListAllTools())

	err = mgr.Stop()
	require.NoError(t, err)
}

func TestManager_StartAndStop_Concurrent(t *testing.T) {
	defer goleak.VerifyNone(t)

	clients := make(map[string]*mockClient)
	var mu sync.Mutex

	cfg := config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"server-a": {Command: "srv-a"},
			"server-b": {Command: "srv-b"},
			"server-c": {URL: "http://localhost:8080/sse"},
		},
	}

	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		mu.Lock()
		defer mu.Unlock()
		c := &mockClient{
			tools: []mcp.Tool{
				{Name: serverName + "_tool_1", Description: "Tool for " + serverName},
			},
		}
		clients[serverName] = c
		return c, nil
	}

	mgr := mcp.NewManager(cfg, mcp.WithClientFactory(clientFactory))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := mgr.Start(ctx)
	require.NoError(t, err)

	activeServers := mgr.Servers()
	assert.Len(t, activeServers, 3)

	allTools := mgr.ListAllTools()
	assert.Len(t, allTools, 3)
	assert.Len(t, allTools["server-a"], 1)
	assert.Equal(t, "server-a_tool_1", allTools["server-a"][0].Name)

	// Verify tool calling routing
	out, err := mgr.CallTool(ctx, "server-a", "server-a_tool_1", nil)
	require.NoError(t, err)
	assert.Equal(t, "result_of_server-a_tool_1", out)

	// Stop manager
	err = mgr.Stop()
	require.NoError(t, err)

	for name, c := range clients {
		assert.True(t, c.closedCalled.Load(), "expected client %s to be closed", name)
	}

	assert.Empty(t, mgr.Servers())
	assert.Empty(t, mgr.ListAllTools())
}

func TestManager_SkipDisabledServers(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"active-server": {Command: "active"},
			"disabled-srv":  {Command: "disabled", Disabled: true},
		},
	}

	mgr := mcp.NewManager(cfg, mcp.WithClientFactory(func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		return &mockClient{
			tools: []mcp.Tool{{Name: serverName + "_tool"}},
		}, nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Start(ctx)
	require.NoError(t, err)
	defer mgr.Stop()

	servers := mgr.Servers()
	assert.Contains(t, servers, "active-server")
	assert.NotContains(t, servers, "disabled-srv")

	tools := mgr.ListAllTools()
	assert.Contains(t, tools, "active-server")
	assert.NotContains(t, tools, "disabled-srv")
}

func TestManager_CallTool_UnknownServer(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"fs": {Command: "fs-srv"},
		},
	}

	mgr := mcp.NewManager(cfg, mcp.WithClientFactory(func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		return &mockClient{}, nil
	}))

	ctx := context.Background()
	require.NoError(t, mgr.Start(ctx))
	defer mgr.Stop()

	_, err := mgr.CallTool(ctx, "non-existent-server", "some_tool", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent-server")
}

func TestManager_Start_FailureCleanup(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"good-server": {Command: "good"},
			"bad-server":  {Command: "bad"},
		},
	}

	var goodClient *mockClient
	mgr := mcp.NewManager(cfg, mcp.WithClientFactory(func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		if serverName == "bad-server" {
			return &mockClient{initErr: errors.New("connection refused")}, nil
		}
		goodClient = &mockClient{}
		return goodClient, nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mgr.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")

	// Ensure good client was closed during cleanup on start failure
	if goodClient != nil {
		assert.True(t, goodClient.closedCalled.Load())
	}
	assert.Empty(t, mgr.Servers())
}
