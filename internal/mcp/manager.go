package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
	"golang.org/x/sync/errgroup"
)

// Manager coordinates the lifecycle, tool aggregation, and routing across multiple MCP servers.
type Manager interface {
	// Start connects and initializes all enabled servers concurrently.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all active MCP client connections.
	Stop() error

	// Servers returns a snapshot of active server clients keyed by server name.
	Servers() map[string]Client

	// ListAllTools returns discovered tools grouped by server name.
	ListAllTools() map[string][]Tool

	// CallTool routes a tool execution to the specified server.
	CallTool(ctx context.Context, serverName, toolName string, args any) (string, error)
}

// ClientFactory instantiates a Client for a specific server configuration.
type ClientFactory func(serverName string, sCfg config.MCPServerConfig) (Client, error)

// TransportFactory instantiates a Transport for a specific server configuration.
type TransportFactory func(serverName string, sCfg config.MCPServerConfig) (transport.Transport, error)

// ManagerOption configures Manager behavior.
type ManagerOption func(*manager)

// WithClientFactory allows overriding client construction (useful for unit testing).
func WithClientFactory(factory ClientFactory) ManagerOption {
	return func(m *manager) {
		m.clientFactory = factory
	}
}

// WithTransportFactory allows overriding transport construction.
func WithTransportFactory(factory TransportFactory) ManagerOption {
	return func(m *manager) {
		m.transportFactory = factory
	}
}

type manager struct {
	cfg              config.MCPConfig
	mu               sync.RWMutex
	clients          map[string]Client
	tools            map[string][]Tool
	clientFactory    ClientFactory
	transportFactory TransportFactory
}

// NewManager creates a new multi-server MCP manager.
func NewManager(cfg config.MCPConfig, opts ...ManagerOption) Manager {
	m := &manager{
		cfg:     cfg,
		clients: make(map[string]Client),
		tools:   make(map[string][]Tool),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.clientFactory == nil {
		m.clientFactory = m.defaultClientFactory
	}
	if m.transportFactory == nil {
		m.transportFactory = m.defaultTransportFactory
	}
	return m
}

func (m *manager) defaultTransportFactory(serverName string, sCfg config.MCPServerConfig) (transport.Transport, error) {
	if sCfg.Command != "" {
		return transport.NewStdio(transport.StdioConfig{
			Command: sCfg.Command,
			Args:    sCfg.Args,
			Env:     sCfg.Env,
		})
	}
	if sCfg.URL != "" {
		return transport.NewSSE(transport.SSEConfig{
			URL: sCfg.URL,
		})
	}
	return nil, fmt.Errorf("server %q configuration must specify either command or url", serverName)
}

func (m *manager) defaultClientFactory(serverName string, sCfg config.MCPServerConfig) (Client, error) {
	tr, err := m.transportFactory(serverName, sCfg)
	if err != nil {
		return nil, err
	}
	return NewClient(tr), nil
}

func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.cfg.Enabled {
		return nil
	}

	type serverResult struct {
		name   string
		client Client
		tools  []Tool
	}

	g, gCtx := errgroup.WithContext(ctx)
	resultsChan := make(chan serverResult, len(m.cfg.Servers))

	for name, sCfg := range m.cfg.Servers {
		if sCfg.Disabled {
			continue
		}

		serverName := name
		serverConfig := sCfg

		g.Go(func() error {
			client, err := m.clientFactory(serverName, serverConfig)
			if err != nil {
				return fmt.Errorf("creating client for server %q: %w", serverName, err)
			}

			if err := client.Initialize(gCtx); err != nil {
				_ = client.Close()
				return fmt.Errorf("initializing server %q: %w", serverName, err)
			}

			tools, err := client.ListTools(gCtx)
			if err != nil {
				_ = client.Close()
				return fmt.Errorf("listing tools for server %q: %w", serverName, err)
			}

			resultsChan <- serverResult{
				name:   serverName,
				client: client,
				tools:  tools,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		close(resultsChan)
		// Clean up any clients that were successfully started before the failure
		for res := range resultsChan {
			_ = res.client.Close()
		}
		return err
	}
	close(resultsChan)

	for res := range resultsChan {
		m.clients[res.name] = res.client
		m.tools[res.name] = res.tools
	}

	return nil
}

func (m *manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var wg sync.WaitGroup
	for _, client := range m.clients {
		c := client
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Close()
		}()
	}
	wg.Wait()

	m.clients = make(map[string]Client)
	m.tools = make(map[string][]Tool)
	return nil
}

func (m *manager) Servers() map[string]Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Client, len(m.clients))
	for k, v := range m.clients {
		result[k] = v
	}
	return result
}

func (m *manager) ListAllTools() map[string][]Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]Tool, len(m.tools))
	for k, v := range m.tools {
		toolsCopy := make([]Tool, len(v))
		copy(toolsCopy, v)
		result[k] = toolsCopy
	}
	return result
}

func (m *manager) CallTool(ctx context.Context, serverName, toolName string, args any) (string, error) {
	m.mu.RLock()
	client, ok := m.clients[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("server %q not found or not running", serverName)
	}

	return client.CallTool(ctx, toolName, args)
}
