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

	defaultDockerImage = "alpine:3"

	// Web tools defaults.
	defaultWebFetchTimeout   = 15 * time.Second
	defaultWebMaxFetchBytes  = 2097152 // 2MB
	defaultWebUserAgent      = "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"
	defaultWebSearchProvider = "duckduckgo"
)

// Config is the root AGIS configuration.
type Config struct {
	LLM     LLMConfig     `yaml:"llm"`
	DB      DBConfig      `yaml:"db"`
	Memory  MemoryConfig  `yaml:"memory"`
	Agent   AgentConfig   `yaml:"agent"`
	Skills  SkillsConfig  `yaml:"skills"`
	Tools   ToolsConfig   `yaml:"tools"`
	Gateway GatewayConfig `yaml:"gateway"`
	Cron    CronConfig    `yaml:"cron"`
	Plugins PluginsConfig  `yaml:"plugins"`
	Webhook WebhookConfig  `yaml:"webhook"`
	Embeddings EmbeddingsConfig `yaml:"embeddings"`
	MCP        MCPConfig        `yaml:"mcp"`
	Multimodal MultimodalConfig `yaml:"multimodal"`
	Subagents  SubagentsConfig  `yaml:"subagents"`
}

// SubagentsConfig gates and tunes the native subagent delegation subsystem.
type SubagentsConfig struct {
	Enabled        bool          `yaml:"enabled"`
	MaxConcurrent  int           `yaml:"max_concurrent"`
	MaxDepth       int           `yaml:"max_depth"`
	DefaultTimeout time.Duration `yaml:"default_timeout"`
	MaxTurns       int           `yaml:"max_turns"`
}

// MultimodalConfig gates and configures the M9 multimodal vision and audio subsystem.
type MultimodalConfig struct {
	Enabled bool         `yaml:"enabled"`
	Vision  VisionConfig `yaml:"vision"`
	Audio   AudioConfig  `yaml:"audio"`
}

// VisionConfig tunes the multimodal vision processing capabilities.
type VisionConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Provider       string `yaml:"provider,omitempty"`
	Model          string `yaml:"model"`
	MaxImageSizeMB int    `yaml:"max_image_size_mb"`
}

// AudioConfig tunes the multimodal audio transcription capabilities.
type AudioConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	MaxAudioSizeMB int    `yaml:"max_audio_size_mb"`
}

// MCPConfig gates and configures the M8 Model Context Protocol client subsystem.
type MCPConfig struct {
	Enabled bool                       `yaml:"enabled"`
	Servers map[string]MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig defines an MCP server endpoint (either local stdio process or remote SSE).
type MCPServerConfig struct {
	Command  string            `yaml:"command,omitempty"`
	Args     []string          `yaml:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	URL      string            `yaml:"url,omitempty"`
	Disabled bool              `yaml:"disabled,omitempty"`
}

// EmbeddingsConfig tunes the M7 hybrid search embeddings subsystem.
type EmbeddingsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Provider   string `yaml:"provider"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
	BatchSize  int    `yaml:"batch_size"`
}

// PluginsConfig gates the M6 plugin subsystem and locates external plugin bundles.
type PluginsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

// WebhookConfig gates the M6 webhook listener subsystem for external event ingestion.
type WebhookConfig struct {
	Enabled          bool                 `yaml:"enabled"`
	Host             string               `yaml:"host"`
	Port             int                  `yaml:"port"`
	Path             string               `yaml:"path"`
	Secret           string               `yaml:"secret"`
	DefaultSessionID string               `yaml:"default_session_id"`
	Target           *WebhookTargetConfig `yaml:"target"`
}

// WebhookTargetConfig configures outbound notification target for webhook responses.
type WebhookTargetConfig struct {
	Adapter   string `yaml:"adapter"`
	Recipient string `yaml:"recipient"`
}

// CronConfig gates the M6 cron scheduling subsystem for periodic tasks.
type CronConfig struct {
	Enabled bool            `yaml:"enabled"`
	Jobs    []CronJobConfig `yaml:"jobs"`
}

// CronJobConfig defines a single scheduled cron job.
type CronJobConfig struct {
	Name      string            `yaml:"name"`
	Schedule  string            `yaml:"schedule"`
	Prompt    string            `yaml:"prompt"`
	SessionID string            `yaml:"session_id"`
	Target    *CronTargetConfig `yaml:"target"`
}

// CronTargetConfig configures outbound notification target for job results.
type CronTargetConfig struct {
	Adapter   string `yaml:"adapter"`
	Recipient string `yaml:"recipient"`
}


// GatewayConfig gates the M6 gateway subsystem for external chat platforms.
type GatewayConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Telegram TelegramConfig `yaml:"telegram"`
	Discord  DiscordConfig  `yaml:"discord"`
}

// TelegramConfig configures the Telegram chat gateway adapter.
type TelegramConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Token     string   `yaml:"token"`
	Allowlist []string `yaml:"allowlist"`
}

// DiscordConfig configures the Discord chat gateway adapter.
type DiscordConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Token     string   `yaml:"token"`
	Allowlist []string `yaml:"allowlist"`
}

// ToolsConfig gates the M4 execution subsystem. Disabled by default: tools
// are strictly opt-in.
type ToolsConfig struct {
	Enabled bool         `yaml:"enabled"`
	Docker  DockerConfig `yaml:"docker"`
	SSH     SSHConfig    `yaml:"ssh"`
	Web     WebConfig    `yaml:"web"`
}

// WebConfig gates and configures the native web tools (web_search, web_fetch).
type WebConfig struct {
	Enabled         bool          `yaml:"enabled"`
	DefaultProvider string        `yaml:"default_provider"` // "duckduckgo", "brave", "tavily", "searxng"
	FetchTimeout    time.Duration `yaml:"fetch_timeout"`   // default: 15s
	MaxFetchBytes   int64         `yaml:"max_fetch_bytes"` // default: 2097152 (2MB)
	UserAgent       string        `yaml:"user_agent"`      // default: "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"
	Providers       WebProviders  `yaml:"providers"`
}

// WebProviders configures credentials and endpoints for web search providers.
type WebProviders struct {
	Brave   ProviderConfig `yaml:"brave"`
	Tavily  ProviderConfig `yaml:"tavily"`
	Searxng ProviderConfig `yaml:"searxng"`

	// Flat field aliases for convenience and backwards compatibility
	BraveAPIKey  string `yaml:"brave_api_key,omitempty"`
	TavilyAPIKey string `yaml:"tavily_api_key,omitempty"`
	SearxngURL   string `yaml:"searxng_url,omitempty"`
}

// GetBraveAPIKey returns the Brave API key from the structured or flat field.
func (p WebProviders) GetBraveAPIKey() string {
	if p.Brave.APIKey != "" {
		return p.Brave.APIKey
	}
	return p.BraveAPIKey
}

// GetTavilyAPIKey returns the Tavily API key from the structured or flat field.
func (p WebProviders) GetTavilyAPIKey() string {
	if p.Tavily.APIKey != "" {
		return p.Tavily.APIKey
	}
	return p.TavilyAPIKey
}

// GetSearxngURL returns the SearXNG BaseURL from the structured or flat field.
func (p WebProviders) GetSearxngURL() string {
	if p.Searxng.BaseURL != "" {
		return p.Searxng.BaseURL
	}
	return p.SearxngURL
}

// ProviderConfig configures a single web search provider endpoint or credentials.
type ProviderConfig struct {
	APIKey  string        `yaml:"api_key,omitempty"`
	BaseURL string        `yaml:"base_url,omitempty"`
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// DockerConfig configures the container backend.
type DockerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Image   string `yaml:"image"`
}

// SSHConfig configures the remote execution backend.
type SSHConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	User    string `yaml:"user"`
	KeyPath string `yaml:"key_path"`
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
	Provider        string        `yaml:"provider,omitempty"`
	Model           string        `yaml:"model,omitempty"`
	RecallLimit     int           `yaml:"recall_limit"`
	NudgeEvery      int           `yaml:"nudge_every"`
	CloseTimeout    time.Duration `yaml:"close_timeout"`
}

// LLMConfig selects the provider and model, and configures credential pools and fallback chains.
type LLMConfig struct {
	Provider  string              `yaml:"provider"`
	Model     string              `yaml:"model"`
	APIKey    string              `yaml:"api_key"`
	APIKeys   []string            `yaml:"api_keys"`
	BaseURL   string              `yaml:"base_url,omitempty"`
	Fallbacks []LLMFallbackConfig `yaml:"fallbacks"`
}

// LLMFallbackConfig configures a fallback LLM provider.
type LLMFallbackConfig struct {
	Provider string   `yaml:"provider"`
	Model    string   `yaml:"model"`
	APIKey   string   `yaml:"api_key"`
	APIKeys  []string `yaml:"api_keys"`
	BaseURL  string   `yaml:"base_url,omitempty"`
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

	path := ResolvePath(configPath)
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
		Tools: ToolsConfig{
			Docker: DockerConfig{Image: defaultDockerImage},
			Web: WebConfig{
				Enabled:         false,
				DefaultProvider: defaultWebSearchProvider,
				FetchTimeout:    defaultWebFetchTimeout,
				MaxFetchBytes:   defaultWebMaxFetchBytes,
				UserAgent:       defaultWebUserAgent,
			},
		},
		Plugins: PluginsConfig{
			Dir: defaultPluginsDir(),
		},
		Webhook: WebhookConfig{
			Host:             "127.0.0.1",
			Port:             8080,
			Path:             "/webhook",
			DefaultSessionID: "webhook-events",
		},
		Embeddings: EmbeddingsConfig{
			Enabled: false,
		},
		MCP: MCPConfig{
			Enabled: false,
			Servers: map[string]MCPServerConfig{},
		},
		Multimodal: MultimodalConfig{
			Enabled: false,
			Vision: VisionConfig{
				Enabled:        false,
				Model:          "llama3.2-vision",
				MaxImageSizeMB: 10,
			},
			Audio: AudioConfig{
				Enabled:        false,
				Provider:       "openai",
				Model:          "whisper-1",
				MaxAudioSizeMB: 25,
			},
		},
		Subagents: SubagentsConfig{
			Enabled:        true,
			MaxConcurrent:  3,
			MaxDepth:       1,
			DefaultTimeout: 60 * time.Second,
			MaxTurns:       8,
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
	if cfg.Plugins.Dir == "" {
		cfg.Plugins.Dir = defaultPluginsDir()
	}
	if cfg.Webhook.Host == "" {
		cfg.Webhook.Host = "127.0.0.1"
	}
	if cfg.Webhook.Port <= 0 {
		cfg.Webhook.Port = 8080
	}
	if cfg.Webhook.Path == "" {
		cfg.Webhook.Path = "/webhook"
	}
	if cfg.Webhook.DefaultSessionID == "" {
		cfg.Webhook.DefaultSessionID = "webhook-events"
	}
	if cfg.Embeddings.Provider == "" {
		cfg.Embeddings.Provider = "ollama"
	}
	if cfg.Embeddings.Model == "" {
		if cfg.Embeddings.Provider == "openai" {
			cfg.Embeddings.Model = "text-embedding-3-small"
		} else {
			cfg.Embeddings.Model = "nomic-embed-text"
		}
	}
	if cfg.Embeddings.Dimensions <= 0 {
		if cfg.Embeddings.Provider == "openai" {
			cfg.Embeddings.Dimensions = 1536
		} else {
			cfg.Embeddings.Dimensions = 768
		}
	}
	if cfg.Embeddings.BatchSize <= 0 {
		cfg.Embeddings.BatchSize = 100
	} else if cfg.Embeddings.BatchSize > 2048 {
		cfg.Embeddings.BatchSize = 2048
	}
	if cfg.Multimodal.Vision.Model == "" {
		cfg.Multimodal.Vision.Model = "llama3.2-vision"
	}
	if cfg.Multimodal.Vision.MaxImageSizeMB <= 0 {
		cfg.Multimodal.Vision.MaxImageSizeMB = 10
	}
	if cfg.Multimodal.Audio.Provider == "" {
		cfg.Multimodal.Audio.Provider = "openai"
	}
	if cfg.Multimodal.Audio.Model == "" {
		cfg.Multimodal.Audio.Model = "whisper-1"
	}
	if cfg.Multimodal.Audio.MaxAudioSizeMB <= 0 {
		cfg.Multimodal.Audio.MaxAudioSizeMB = 25
	}
	if cfg.Tools.Web.DefaultProvider == "" {
		cfg.Tools.Web.DefaultProvider = defaultWebSearchProvider
	}
	if cfg.Tools.Web.FetchTimeout <= 0 {
		cfg.Tools.Web.FetchTimeout = defaultWebFetchTimeout
	}
	if cfg.Tools.Web.MaxFetchBytes <= 0 {
		cfg.Tools.Web.MaxFetchBytes = defaultWebMaxFetchBytes
	}
	if cfg.Tools.Web.UserAgent == "" {
		cfg.Tools.Web.UserAgent = defaultWebUserAgent
	}
	if cfg.Subagents.MaxConcurrent <= 0 {
		cfg.Subagents.MaxConcurrent = 1
	} else if cfg.Subagents.MaxConcurrent > 10 {
		cfg.Subagents.MaxConcurrent = 10
	}
	if cfg.Subagents.MaxDepth <= 0 {
		cfg.Subagents.MaxDepth = 1
	} else if cfg.Subagents.MaxDepth > 2 {
		cfg.Subagents.MaxDepth = 2
	}
	if cfg.Subagents.MaxTurns <= 0 {
		cfg.Subagents.MaxTurns = 8
	} else if cfg.Subagents.MaxTurns > 15 {
		cfg.Subagents.MaxTurns = 15
	}
	if cfg.Subagents.DefaultTimeout <= 0 {
		cfg.Subagents.DefaultTimeout = 60 * time.Second
	} else if cfg.Subagents.DefaultTimeout > 300*time.Second {
		cfg.Subagents.DefaultTimeout = 300 * time.Second
	}
}

// defaultPluginsDir returns $AGIS_HOME/plugins (or ~/.agis/plugins) for the active profile.
func defaultPluginsDir() string {
	return filepath.Join(CurrentProfileDir(), "plugins")
}

// AgisHome exposes the resolved AGIS home directory ($AGIS_HOME or
// ~/.agis) scoped to the active profile context. Identity, skills, and the registry live here.
func AgisHome() string {
	return CurrentProfileDir()
}

// defaultSkillsDir returns $AGIS_HOME/skills (or ~/.agis/skills) for the active profile.
func defaultSkillsDir() string {
	return filepath.Join(CurrentProfileDir(), skillsDirName)
}

// ResolvePath applies the -config flag > AGIS_HOME > default precedence scoped to the active profile.
func ResolvePath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	return filepath.Join(CurrentProfileDir(), configFileName)
}

// agisDir returns the AGIS base directory: AGIS_HOME when set, otherwise
// ~/.agis.
func agisDir() string {
	return BaseHome()
}

func defaultDBPath() string {
	return filepath.Join(CurrentProfileDir(), dbFileName)
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
