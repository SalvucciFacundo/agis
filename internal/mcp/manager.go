package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
	"golang.org/x/sync/errgroup"
)

// ServerState represents the lifecycle status of an MCP server.
type ServerState string

const (
	StateStandby  ServerState = "standby"
	StateStarting ServerState = "starting"
	StateRunning  ServerState = "running"
	StateStopping ServerState = "stopping"
	StateDisabled ServerState = "disabled"
)

// ServerStatus describes the live state of an MCP server.
type ServerStatus struct {
	Name         string      `json:"name"`
	State        ServerState `json:"state"`
	ToolCount    int         `json:"tool_count"`
	LastActivity time.Time   `json:"last_activity,omitempty"`
	Transport    string      `json:"transport"`
}

// Manager coordinates the lifecycle, tool aggregation, and routing across multiple MCP servers.
type Manager interface {
	// Start connects and initializes enabled servers or populates schema caches for Standby mode.
	Start(ctx context.Context) error

	// Stop gracefully shuts down all active MCP client connections.
	Stop() error

	// Servers returns a snapshot of active server clients keyed by server name.
	Servers() map[string]Client

	// ListAllTools returns discovered tools grouped by server name.
	ListAllTools() map[string][]Tool

	// CallTool routes a tool execution to the specified server, waking it from Standby on demand.
	CallTool(ctx context.Context, serverName, toolName string, args any) (string, error)

	// ServerStatus returns the lifecycle status of a specific server.
	ServerStatus(serverName string) (ServerStatus, bool)

	// RefreshTools forces re-querying tools for a server, updating disk cache and memory.
	RefreshTools(ctx context.Context, serverName string) ([]Tool, error)
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

// WithSchemaCache allows overriding the schema cache implementation.
func WithSchemaCache(cache SchemaCache) ManagerOption {
	return func(m *manager) {
		m.cache = cache
	}
}

type serverSession struct {
	mu           sync.Mutex
	name         string
	cfg          config.MCPServerConfig
	state        ServerState
	client       Client
	tools        []Tool
	idleTimer    *time.Timer
	lastActivity time.Time
	idleTimeout  time.Duration
	isStandby    bool
	inFlight     atomic.Int64
}

type manager struct {
	cfg              config.MCPConfig
	mu               sync.RWMutex
	sessions         map[string]*serverSession
	clientFactory    ClientFactory
	transportFactory TransportFactory
	cache            SchemaCache
	stopped          atomic.Bool
}

// NewManager creates a new multi-server MCP manager with Standby and on-demand capabilities.
func NewManager(cfg config.MCPConfig, opts ...ManagerOption) Manager {
	m := &manager{
		cfg:      cfg,
		sessions: make(map[string]*serverSession),
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
	if m.cache == nil {
		m.cache = NewDiskSchemaCache(cfg.CacheDir)
	}

	for name, sCfg := range cfg.Servers {
		state := StateStandby
		if sCfg.Disabled {
			state = StateDisabled
		}
		m.sessions[name] = &serverSession{
			name:        name,
			cfg:         sCfg,
			state:       state,
			idleTimeout: sCfg.ParsedIdleTimeout(cfg.ParsedIdleTimeout()),
			isStandby:   sCfg.IsStandby(cfg.Standby),
		}
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

	if !m.cfg.Enabled || len(m.sessions) == 0 {
		return nil
	}

	g, gCtx := errgroup.WithContext(ctx)
	type probeResult struct {
		sess   *serverSession
		client Client
		tools  []Tool
	}
	resultsChan := make(chan probeResult, len(m.sessions))

	for _, sess := range m.sessions {
		if sess.cfg.Disabled {
			continue
		}

		s := sess
		if s.isStandby {
			// Check disk cache first
			tools, ok, err := m.cache.Load(s.name)
			if err == nil && ok && len(tools) > 0 {
				s.tools = tools
				s.state = StateStandby
				continue
			}

			// Cache miss: execute transient probe to populate schema cache
			g.Go(func() error {
				client, err := m.clientFactory(s.name, s.cfg)
				if err != nil {
					return fmt.Errorf("creating client for server %q: %w", s.name, err)
				}

				if err := client.Initialize(gCtx); err != nil {
					_ = client.Close()
					return fmt.Errorf("initializing server %q: %w", s.name, err)
				}

				tools, err := client.ListTools(gCtx)
				if err != nil {
					_ = client.Close()
					return fmt.Errorf("listing tools for server %q: %w", s.name, err)
				}

				_ = client.Close()
				_ = m.cache.Save(s.name, tools)

				resultsChan <- probeResult{
					sess:   s,
					client: nil,
					tools:  tools,
				}
				return nil
			})
		} else {
			// Eager mode: start server and keep client active
			g.Go(func() error {
				client, err := m.clientFactory(s.name, s.cfg)
				if err != nil {
					return fmt.Errorf("creating client for server %q: %w", s.name, err)
				}

				if err := client.Initialize(gCtx); err != nil {
					_ = client.Close()
					return fmt.Errorf("initializing server %q: %w", s.name, err)
				}

				tools, err := client.ListTools(gCtx)
				if err != nil {
					_ = client.Close()
					return fmt.Errorf("listing tools for server %q: %w", s.name, err)
				}

				_ = m.cache.Save(s.name, tools)

				resultsChan <- probeResult{
					sess:   s,
					client: client,
					tools:  tools,
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		close(resultsChan)
		for res := range resultsChan {
			if res.client != nil {
				_ = res.client.Close()
			}
		}
		return err
	}
	close(resultsChan)

	for res := range resultsChan {
		res.sess.tools = res.tools
		if res.client != nil {
			res.sess.client = res.client
			res.sess.state = StateRunning
			res.sess.lastActivity = time.Now()
		} else {
			res.sess.state = StateStandby
		}
	}

	return nil
}

func (m *manager) CallTool(ctx context.Context, serverName, toolName string, args any) (string, error) {
	if m.stopped.Load() {
		return "", fmt.Errorf("mcp manager is stopped")
	}

	m.mu.RLock()
	sess, ok := m.sessions[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("server %q not found or not running", serverName)
	}
	if sess.cfg.Disabled {
		return "", fmt.Errorf("server %q is disabled", serverName)
	}

	sess.mu.Lock()
	if sess.client == nil {
		sess.state = StateStarting
		client, err := m.clientFactory(sess.name, sess.cfg)
		if err != nil {
			sess.state = StateStandby
			sess.mu.Unlock()
			return "", fmt.Errorf("creating client for server %q: %w", serverName, err)
		}
		if err := client.Initialize(ctx); err != nil {
			_ = client.Close()
			sess.state = StateStandby
			sess.mu.Unlock()
			return "", fmt.Errorf("initializing server %q: %w", serverName, err)
		}
		if len(sess.tools) == 0 {
			tools, err := client.ListTools(ctx)
			if err == nil {
				sess.tools = tools
				_ = m.cache.Save(sess.name, tools)
			}
		}
		sess.client = client
		sess.state = StateRunning
	}

	if sess.idleTimer != nil {
		sess.idleTimer.Stop()
		sess.idleTimer = nil
	}
	sess.lastActivity = time.Now()

	if sess.isStandby && sess.idleTimeout > 0 {
		sess.idleTimer = time.AfterFunc(sess.idleTimeout, func() {
			m.shutdownIdleSession(sess)
		})
	}

	client := sess.client
	sess.inFlight.Add(1)
	sess.mu.Unlock()

	defer func() {
		sess.inFlight.Add(-1)
		sess.mu.Lock()
		sess.lastActivity = time.Now()
		sess.mu.Unlock()
	}()

	return client.CallTool(ctx, toolName, args)
}

func (m *manager) shutdownIdleSession(sess *serverSession) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.client == nil || m.stopped.Load() {
		return
	}

	if sess.inFlight.Load() > 0 {
		sess.idleTimer = time.AfterFunc(sess.idleTimeout, func() {
			m.shutdownIdleSession(sess)
		})
		return
	}

	elapsed := time.Since(sess.lastActivity)
	if elapsed < sess.idleTimeout {
		sess.idleTimer = time.AfterFunc(sess.idleTimeout-elapsed, func() {
			m.shutdownIdleSession(sess)
		})
		return
	}

	_ = sess.client.Close()
	sess.client = nil
	sess.idleTimer = nil
	sess.state = StateStandby
}

func (m *manager) Stop() error {
	m.stopped.Store(true)
	m.mu.Lock()
	defer m.mu.Unlock()

	var wg sync.WaitGroup
	for _, sess := range m.sessions {
		s := sess
		s.mu.Lock()
		if s.idleTimer != nil {
			s.idleTimer.Stop()
			s.idleTimer = nil
		}
		client := s.client
		s.client = nil
		s.tools = nil
		s.state = StateStandby
		if s.cfg.Disabled {
			s.state = StateDisabled
		}
		s.mu.Unlock()

		if client != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = client.Close()
			}()
		}
	}
	wg.Wait()
	return nil
}

func (m *manager) Servers() map[string]Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Client)
	for name, sess := range m.sessions {
		sess.mu.Lock()
		if sess.client != nil {
			result[name] = sess.client
		}
		sess.mu.Unlock()
	}
	return result
}

func (m *manager) ListAllTools() map[string][]Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped.Load() {
		return make(map[string][]Tool)
	}

	result := make(map[string][]Tool, len(m.sessions))
	for name, sess := range m.sessions {
		sess.mu.Lock()
		if len(sess.tools) > 0 {
			toolsCopy := make([]Tool, len(sess.tools))
			copy(toolsCopy, sess.tools)
			result[name] = toolsCopy
		}
		sess.mu.Unlock()
	}
	return result
}

func (m *manager) ServerStatus(serverName string) (ServerStatus, bool) {
	m.mu.RLock()
	sess, ok := m.sessions[serverName]
	m.mu.RUnlock()
	if !ok {
		return ServerStatus{}, false
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	tr := "stdio"
	if sess.cfg.URL != "" {
		tr = "sse"
	}

	return ServerStatus{
		Name:         sess.name,
		State:        sess.state,
		ToolCount:    len(sess.tools),
		LastActivity: sess.lastActivity,
		Transport:    tr,
	}, true
}

func (m *manager) RefreshTools(ctx context.Context, serverName string) ([]Tool, error) {
	m.mu.RLock()
	sess, ok := m.sessions[serverName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("server %q not found", serverName)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	_ = m.cache.Invalidate(serverName)

	wasRunning := sess.client != nil
	client := sess.client
	if client == nil {
		var err error
		client, err = m.clientFactory(sess.name, sess.cfg)
		if err != nil {
			return nil, fmt.Errorf("creating client for server %q: %w", serverName, err)
		}
		if err := client.Initialize(ctx); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("initializing server %q: %w", serverName, err)
		}
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		if !wasRunning {
			_ = client.Close()
		}
		return nil, fmt.Errorf("listing tools for server %q: %w", serverName, err)
	}

	_ = m.cache.Save(serverName, tools)
	sess.tools = tools

	if !wasRunning {
		_ = client.Close()
		sess.client = nil
		sess.state = StateStandby
	}

	toolsCopy := make([]Tool, len(tools))
	copy(toolsCopy, tools)
	return toolsCopy, nil
}
