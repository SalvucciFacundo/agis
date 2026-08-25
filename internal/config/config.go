// Package config loads AGIS configuration from YAML.
//
// Resolution precedence (highest first): the -config flag, the AGIS_HOME
// environment variable, and finally the default ~/.agis/config.yaml path.
// A missing file is not an error: built-in defaults are used. The file is
// expected to be mode 0600; a looser mode produces a warning on stderr.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultProvider = "ollama"
	defaultModel    = "llama3.2"
	configFileName  = "config.yaml"
	dbFileName      = "agis.db"
	dotAgisDir      = ".agis"
	// expectedPerm is the required config file mode.
	expectedPerm = 0o600

	// Learning-loop defaults for the memory block.
	defaultRecallLimit  = 10
	defaultNudgeEvery   = 10
	defaultCloseTimeout = 30 * time.Second

	skillsDirName = "skills"
)

// Config is the root AGIS configuration.
type Config struct {
	LLM    LLMConfig    `yaml:"llm"`
	DB     DBConfig     `yaml:"db"`
	Memory MemoryConfig `yaml:"memory"`
	Agent  AgentConfig  `yaml:"agent"`
	Skills SkillsConfig `yaml:"skills"`
}

// AgentConfig carries identity and persona settings: custom personality
// presets and whether persona evolution participates in prompts.
type AgentConfig struct {
	Personalities    map[string]string `yaml:"personalities"`
	EvolutionEnabled bool              `yaml:"evolution_enabled"`
}

// SkillsConfig tunes the skill hub: master switch and where skill files live.
type SkillsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

// MemoryConfig tunes the M2 learning loop: whether curation runs at all, the
// top-N recall bound, the nudge cadence, and how long a session close may take.
type MemoryConfig struct {
	LearningEnabled bool          `yaml:"learning_enabled"`
	RecallLimit     int           `yaml:"recall_limit"`
	NudgeEvery      int           `yaml:"nudge_every"`
	CloseTimeout    time.Duration `yaml:"close_timeout"`
}

// LLMConfig selects the provider and model.
type LLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
}

// DBConfig locates the SQLite database file.
type DBConfig struct {
	Path string `yaml:"path"`
}

// Option configures the Load behavior.
type Option func(*loadOptions)

type loadOptions struct {
	warnWriter io.Writer
}

// WithWarnWriter redirects configuration warnings (e.g. loose permissions)
// to w instead of os.Stderr. Useful in tests.
func WithWarnWriter(w io.Writer) Option {
	return func(o *loadOptions) { o.warnWriter = w }
}

// Load resolves the config file path, applies defaults, overlays the file
// contents when present, and warns if the file mode is looser than 0600.
func Load(configPath string, opts ...Option) (*Config, error) {
	o := &loadOptions{warnWriter: os.Stderr}
	for _, opt := range opts {
		opt(o)
	}

	path := resolvePath(configPath)
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	warnPerms(o.warnWriter, path)

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider: defaultProvider,
			Model:    defaultModel,
		},
		DB: DBConfig{
			Path: defaultDBPath(),
		},
		Memory: MemoryConfig{
			LearningEnabled: true,
			RecallLimit:     defaultRecallLimit,
			NudgeEvery:      defaultNudgeEvery,
			CloseTimeout:    defaultCloseTimeout,
		},
		Agent: AgentConfig{
			EvolutionEnabled: true,
		},
		Skills: SkillsConfig{
			Enabled: true,
			Dir:     defaultSkillsDir(),
		},
	}
}

// applyDefaults restores defaults for fields left empty by the config file.
// APIKey is intentionally untouched: an empty key is a valid value.
// LearningEnabled and NudgeEvery are intentionally untouched too: an explicit
// false (learning off) and an explicit zero (nudging off) are valid values,
// and yaml.Unmarshal only overlays keys present in the file, so absent keys
// keep their defaults.
func applyDefaults(cfg *Config) {
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = defaultProvider
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = defaultModel
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = defaultDBPath()
	}
	if cfg.Memory.RecallLimit <= 0 {
		cfg.Memory.RecallLimit = defaultRecallLimit
	}
	if cfg.Memory.CloseTimeout <= 0 {
		cfg.Memory.CloseTimeout = defaultCloseTimeout
	}
	if cfg.Skills.Dir == "" {
		cfg.Skills.Dir = defaultSkillsDir()
	}
}

// AgisHome exposes the resolved AGIS home directory ($AGIS_HOME or
// ~/.agis). Identity, skills, and the registry live here.
func AgisHome() string {
	return agisDir()
}

// defaultSkillsDir returns $AGIS_HOME/skills (or ~/.agis/skills).
func defaultSkillsDir() string {
	return filepath.Join(agisDir(), skillsDirName)
}

// resolvePath applies the -config flag > AGIS_HOME > default precedence.
func resolvePath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	return filepath.Join(agisDir(), configFileName)
}

// agisDir returns the AGIS base directory: AGIS_HOME when set, otherwise
// ~/.agis.
func agisDir() string {
	if home := os.Getenv("AGIS_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return dotAgisDir
	}
	return filepath.Join(userHome, dotAgisDir)
}

func defaultDBPath() string {
	return filepath.Join(agisDir(), dbFileName)
}

// warnPerms emits a warning when path's mode grants any permission to group
// or other (i.e. looser than 0600).
func warnPerms(w io.Writer, path string) {
	if w == nil {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		fmt.Fprintf(w, "agis: warning: %s has permissions %04o; expected %04o\n",
			path, perm, expectedPerm)
	}
}
