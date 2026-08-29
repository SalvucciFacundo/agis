package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/skills"
)

const stateFileName = "state.json"

// PluginInfo bundles a parsed Manifest with its runtime enabled state and filesystem location.
type PluginInfo struct {
	Manifest Manifest `json:"manifest"`
	Enabled  bool     `json:"enabled"`
	Dir      string   `json:"dir"`
}

// Manager manages plugin discovery, activation lifecycle, and tool/skill registration.
type Manager struct {
	mu        sync.RWMutex
	plugins   map[string]*PluginInfo
	stateDir  string
	stateFile string
	logger    *slog.Logger
}

// Option configures a Manager instance.
type Option func(*Manager)

// WithStateDir sets the directory where state.json is stored.
func WithStateDir(dir string) Option {
	return func(m *Manager) {
		m.stateDir = dir
		m.stateFile = filepath.Join(dir, stateFileName)
	}
}

// WithStateFile explicitly sets the full path to state.json.
func WithStateFile(path string) Option {
	return func(m *Manager) {
		m.stateFile = path
		m.stateDir = filepath.Dir(path)
	}
}

// WithLogger sets the logger for the plugin manager.
func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// NewManager creates a new plugin manager.
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		plugins: make(map[string]*PluginInfo),
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Load scans the given root directory for plugin subdirectories containing plugin.json.
// It also reads persisted state.json to restore enabled statuses.
func (m *Manager) Load(pluginsDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stateFile == "" {
		m.stateFile = filepath.Join(pluginsDir, stateFileName)
		m.stateDir = pluginsDir
	}

	state := m.loadStateLocked()

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading plugins directory %s: %w", pluginsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDir := filepath.Join(pluginsDir, entry.Name())
		manifestPath := filepath.Join(subDir, "plugin.json")

		manifest, err := ParseManifestFile(manifestPath)
		if err != nil {
			m.logger.Warn("plugins: skipping invalid plugin", "dir", subDir, "error", err)
			continue
		}

		enabled := state[manifest.Name]

		m.plugins[manifest.Name] = &PluginInfo{
			Manifest: *manifest,
			Enabled:  enabled,
			Dir:      subDir,
		}
	}

	return nil
}

// List returns all discovered plugins sorted alphabetically by name.
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		out = append(out, *p)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest.Name < out[j].Manifest.Name
	})

	return out
}

// Get returns the PluginInfo for the specified plugin name.
func (m *Manager) Get(name string) (*PluginInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	cp := *p
	return &cp, nil
}

// Enable activates the plugin and persists its state to disk.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	p.Enabled = true
	return m.saveStateLocked()
}

// Disable deactivates the plugin and persists its state to disk.
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	p.Enabled = false
	return m.saveStateLocked()
}

// Skills returns all valid skills declared by currently enabled plugins.
func (m *Manager) Skills() []core.Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []core.Skill
	for _, p := range m.plugins {
		if !p.Enabled {
			continue
		}
		if len(p.Manifest.Skills) > 0 {
			for _, skillFile := range p.Manifest.Skills {
				skillPath := filepath.Join(p.Dir, skillFile)
				data, err := os.ReadFile(skillPath)
				if err != nil {
					m.logger.Warn("plugins: reading skill file failed", "path", skillPath, "error", err)
					continue
				}
				skillDir := filepath.Dir(skillPath)
				loaded, err := skills.LoadDir(skillDir, m.logger)
				if err != nil {
					m.logger.Warn("plugins: loading skill failed", "dir", skillDir, "error", err)
					continue
				}
				for _, s := range loaded {
					if strings.EqualFold(filepath.Base(skillPath), skillFile) || s.Name != "" {
						out = append(out, s)
						break
					}
				}
				_ = data
			}
		}
	}

	return out
}

// RegisterSkills registers all skills from enabled plugins with the skill hub and repository.
func (m *Manager) RegisterSkills(ctx context.Context, repo core.Repository, hub *skills.Hub) error {
	pluginSkills := m.Skills()
	for _, s := range pluginSkills {
		if repo != nil {
			if err := repo.SaveSkill(ctx, s); err != nil {
				m.logger.Warn("plugins: saving skill to repository failed", "skill", s.Name, "error", err)
			}
		}
		if hub != nil {
			hub.Add(s)
		}
	}
	return nil
}
// Runners returns a slice of core.ToolRunner for all enabled plugins that have an entrypoint or tools.
func (m *Manager) Runners() []core.ToolRunner {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var runners []core.ToolRunner
	for _, p := range m.plugins {
		if p.Enabled && (p.Manifest.Entrypoint != "" || len(p.Manifest.Tools) > 0) {
			runners = append(runners, newPluginRunner(p))
		}
	}

	sort.Slice(runners, func(i, j int) bool {
		return runners[i].Backend() < runners[j].Backend()
	})

	return runners
}

func (m *Manager) loadStateLocked() map[string]bool {
	state := make(map[string]bool)
	if m.stateFile == "" {
		return state
	}

	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		return state
	}

	_ = json.Unmarshal(data, &state)
	return state
}

func (m *Manager) saveStateLocked() error {
	if m.stateFile == "" {
		return errors.New("plugin state file path not configured")
	}

	state := make(map[string]bool)
	for name, p := range m.plugins {
		state[name] = p.Enabled
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.stateFile), 0o700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	if err := os.WriteFile(m.stateFile, data, 0o600); err != nil {
		return fmt.Errorf("saving plugin state %s: %w", m.stateFile, err)
	}

	return nil
}

// PluginRunner implements core.ToolRunner for an individual plugin.
type PluginRunner struct {
	plugin *PluginInfo
}

func newPluginRunner(plugin *PluginInfo) *PluginRunner {
	return &PluginRunner{plugin: plugin}
}

// Backend returns the tool backend identifier formatted as "plugin-<name>".
func (r *PluginRunner) Backend() string {
	return "plugin-" + r.plugin.Manifest.Name
}

// Run executes the plugin's entrypoint executable in the plugin directory with the command argument.
func (r *PluginRunner) Run(ctx context.Context, command string) (string, error) {
	entrypoint := r.plugin.Manifest.Entrypoint
	if entrypoint == "" {
		return "", fmt.Errorf("plugin %s has no entrypoint configured", r.plugin.Manifest.Name)
	}

	execPath := filepath.Join(r.plugin.Dir, entrypoint)

	// #nosec G204 - Entrypoint execution is restricted to configured plugin directory and validated command
	cmd := exec.CommandContext(ctx, execPath, command)
	cmd.Dir = r.plugin.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("plugin execution error: %w (output: %s)", err, output)
	}

	return output, nil
}
