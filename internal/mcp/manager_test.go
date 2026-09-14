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

func TestManager_Standby_WarmCacheNoSubprocess(t *testing.T) {
	defer goleak.VerifyNone(t)

	cacheDir := t.TempDir()
	cache := mcp.NewDiskSchemaCache(cacheDir)
	err := cache.Save("warm-srv", []mcp.Tool{
		{Name: "cached_tool", Description: "Cached tool description"},
	})
	require.NoError(t, err)

	cfg := config.MCPConfig{
		Enabled:  true,
		Standby:  true,
		CacheDir: cacheDir,
		Servers: map[string]config.MCPServerConfig{
			"warm-srv": {Command: "echo"},
		},
	}

	var factoryCalls atomic.Int32
	var clientCreated *mockClient
	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		factoryCalls.Add(1)
		clientCreated = &mockClient{
			tools: []mcp.Tool{
				{Name: "cached_tool", Description: "Cached tool description"},
			},
		}
		return clientCreated, nil
	}

	mgr := mcp.NewManager(cfg,
		mcp.WithClientFactory(clientFactory),
		mcp.WithSchemaCache(cache),
	)

	ctx := context.Background()
	err = mgr.Start(ctx)
	require.NoError(t, err)

	// Zero clients spawned at boot time
	assert.Equal(t, int32(0), factoryCalls.Load())
	assert.Empty(t, mgr.Servers())

	// Tools are immediately available from cache
	toolsMap := mgr.ListAllTools()
	require.Len(t, toolsMap["warm-srv"], 1)
	assert.Equal(t, "cached_tool", toolsMap["warm-srv"][0].Name)

	status, ok := mgr.ServerStatus("warm-srv")
	require.True(t, ok)
	assert.Equal(t, mcp.StateStandby, status.State)

	// Invoke CallTool: triggers on-demand activation
	res, err := mgr.CallTool(ctx, "warm-srv", "cached_tool", nil)
	require.NoError(t, err)
	assert.Equal(t, "result_of_cached_tool", res)
	assert.Equal(t, int32(1), factoryCalls.Load())

	status, ok = mgr.ServerStatus("warm-srv")
	require.True(t, ok)
	assert.Equal(t, mcp.StateRunning, status.State)

	err = mgr.Stop()
	require.NoError(t, err)
	assert.True(t, clientCreated.closedCalled.Load())
}

func TestManager_Standby_ColdCacheTransientProbe(t *testing.T) {
	defer goleak.VerifyNone(t)

	cacheDir := t.TempDir()
	cache := mcp.NewDiskSchemaCache(cacheDir)

	cfg := config.MCPConfig{
		Enabled:  true,
		Standby:  true,
		CacheDir: cacheDir,
		Servers: map[string]config.MCPServerConfig{
			"cold-srv": {Command: "echo"},
		},
	}

	var clientCreated *mockClient
	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		clientCreated = &mockClient{
			tools: []mcp.Tool{
				{Name: "probed_tool", Description: "Discovered during probe"},
			},
		}
		return clientCreated, nil
	}

	mgr := mcp.NewManager(cfg,
		mcp.WithClientFactory(clientFactory),
		mcp.WithSchemaCache(cache),
	)

	ctx := context.Background()
	err := mgr.Start(ctx)
	require.NoError(t, err)

	// Client was probed and immediately closed back to Standby
	require.NotNil(t, clientCreated)
	assert.True(t, clientCreated.closedCalled.Load())
	assert.Empty(t, mgr.Servers())

	status, ok := mgr.ServerStatus("cold-srv")
	require.True(t, ok)
	assert.Equal(t, mcp.StateStandby, status.State)

	// Cached to disk
	cachedTools, ok, err := cache.Load("cold-srv")
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, cachedTools, 1)
	assert.Equal(t, "probed_tool", cachedTools[0].Name)

	err = mgr.Stop()
	require.NoError(t, err)
}

func TestManager_Standby_InactivityWatchdogAutoShutdown(t *testing.T) {
	defer goleak.VerifyNone(t)

	cacheDir := t.TempDir()
	cache := mcp.NewDiskSchemaCache(cacheDir)
	_ = cache.Save("idle-srv", []mcp.Tool{{Name: "ping"}})

	cfg := config.MCPConfig{
		Enabled:     true,
		Standby:     true,
		IdleTimeout: "50ms",
		CacheDir:    cacheDir,
		Servers: map[string]config.MCPServerConfig{
			"idle-srv": {Command: "echo"},
		},
	}

	var clientCreated *mockClient
	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		clientCreated = &mockClient{
			tools: []mcp.Tool{{Name: "ping"}},
		}
		return clientCreated, nil
	}

	mgr := mcp.NewManager(cfg,
		mcp.WithClientFactory(clientFactory),
		mcp.WithSchemaCache(cache),
	)

	ctx := context.Background()
	err := mgr.Start(ctx)
	require.NoError(t, err)

	// Trigger on-demand spawn
	_, err = mgr.CallTool(ctx, "idle-srv", "ping", nil)
	require.NoError(t, err)
	assert.False(t, clientCreated.closedCalled.Load())

	// Wait for idle watchdog timeout (50ms timeout -> wait ~100ms)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, clientCreated.closedCalled.Load())
	status, ok := mgr.ServerStatus("idle-srv")
	require.True(t, ok)
	assert.Equal(t, mcp.StateStandby, status.State)

	err = mgr.Stop()
	require.NoError(t, err)
}

func TestManager_Standby_ConcurrentCalls(t *testing.T) {
	defer goleak.VerifyNone(t)

	cacheDir := t.TempDir()
	cache := mcp.NewDiskSchemaCache(cacheDir)
	_ = cache.Save("conc-srv", []mcp.Tool{{Name: "worker"}})

	cfg := config.MCPConfig{
		Enabled:     true,
		Standby:     true,
		IdleTimeout: "1s",
		CacheDir:    cacheDir,
		Servers: map[string]config.MCPServerConfig{
			"conc-srv": {Command: "echo"},
		},
	}

	var factoryCalls atomic.Int32
	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		factoryCalls.Add(1)
		time.Sleep(10 * time.Millisecond) // simulate startup latency
		return &mockClient{
			tools: []mcp.Tool{{Name: "worker"}},
		}, nil
	}

	mgr := mcp.NewManager(cfg,
		mcp.WithClientFactory(clientFactory),
		mcp.WithSchemaCache(cache),
	)

	ctx := context.Background()
	err := mgr.Start(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := mgr.CallTool(ctx, "conc-srv", "worker", nil)
			assert.NoError(t, err)
			assert.Equal(t, "result_of_worker", res)
		}()
	}
	wg.Wait()

	// Exactly 1 client created despite 10 concurrent requests
	assert.Equal(t, int32(1), factoryCalls.Load())

	err = mgr.Stop()
	require.NoError(t, err)
}

func TestManager_RefreshTools(t *testing.T) {
	defer goleak.VerifyNone(t)

	cacheDir := t.TempDir()
	cache := mcp.NewDiskSchemaCache(cacheDir)
	_ = cache.Save("ref-srv", []mcp.Tool{{Name: "old_tool"}})

	cfg := config.MCPConfig{
		Enabled:  true,
		Standby:  true,
		CacheDir: cacheDir,
		Servers: map[string]config.MCPServerConfig{
			"ref-srv": {Command: "echo"},
		},
	}

	clientFactory := func(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
		return &mockClient{
			tools: []mcp.Tool{{Name: "new_tool", Description: "Updated tool"}},
		}, nil
	}

	mgr := mcp.NewManager(cfg,
		mcp.WithClientFactory(clientFactory),
		mcp.WithSchemaCache(cache),
	)

	ctx := context.Background()
	err := mgr.Start(ctx)
	require.NoError(t, err)

	// Before refresh: old tool
	assert.Equal(t, "old_tool", mgr.ListAllTools()["ref-srv"][0].Name)

	// Refresh
	tools, err := mgr.RefreshTools(ctx, "ref-srv")
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "new_tool", tools[0].Name)

	// In memory updated
	assert.Equal(t, "new_tool", mgr.ListAllTools()["ref-srv"][0].Name)

	// On disk updated
	cached, ok, err := cache.Load("ref-srv")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "new_tool", cached[0].Name)

	err = mgr.Stop()
	require.NoError(t, err)
}
