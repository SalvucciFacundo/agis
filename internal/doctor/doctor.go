package doctor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/persona"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/skills"
	_ "modernc.org/sqlite"
)

// CheckStatus represents the outcome of an individual diagnostic check.
type CheckStatus string

const (
	StatusPass CheckStatus = "PASS"
	StatusWarn CheckStatus = "WARN"
	StatusFail CheckStatus = "FAIL"
)

// CheckResult records the execution and findings of a single health probe.
type CheckResult struct {
	Name     string        `json:"name"`
	Title    string        `json:"title"`
	Status   CheckStatus   `json:"status"`
	Message  string        `json:"message"`
	Details  []string      `json:"details,omitempty"`
	Duration time.Duration `json:"duration_ms"`
}

// Report holds the aggregated findings of all diagnostic checks.
type Report struct {
	Results   []CheckResult `json:"results"`
	Timestamp time.Time     `json:"timestamp"`
	Summary   ReportSummary `json:"summary"`
}

// ReportSummary summarizes check counts.
type ReportSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
}

// HasFailures returns true if any check returned StatusFail.
func (r *Report) HasFailures() bool {
	return r.Summary.Failed > 0
}

// HasWarnings returns true if any check returned StatusWarn.
func (r *Report) HasWarnings() bool {
	return r.Summary.Warnings > 0
}

// Find locates a result by check name.
func (r *Report) Find(name string) *CheckResult {
	for i := range r.Results {
		if r.Results[i].Name == name {
			return &r.Results[i]
		}
	}
	return nil
}

// JSON serializes the report to indented JSON bytes.
func (r *Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Option configures Doctor execution.
type Option func(*Doctor)

// WithHTTPClient overrides the HTTP client used for connectivity checks.
func WithHTTPClient(client *http.Client) Option {
	return func(d *Doctor) {
		d.httpClient = client
	}
}

// WithAgisHome overrides the resolved AGIS_HOME path.
func WithAgisHome(dir string) Option {
	return func(d *Doctor) {
		d.agisHome = dir
	}
}

// WithOllamaURL overrides the base URL used to probe Ollama.
func WithOllamaURL(url string) Option {
	return func(d *Doctor) {
		d.ollamaURL = url
	}
}

// WithOpenAIBaseURL overrides the base URL used to probe OpenAI-compatible providers.
func WithOpenAIBaseURL(url string) Option {
	return func(d *Doctor) {
		d.openAIBaseURL = url
	}
}

// Doctor manages system diagnostics.
type Doctor struct {
	cfg           *config.Config
	httpClient    *http.Client
	agisHome      string
	ollamaURL     string
	openAIBaseURL string
}

// New constructs a Doctor probe suite.
func New(cfg *config.Config, opts ...Option) *Doctor {
	d := &Doctor{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		agisHome: config.AgisHome(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Run executes all health probes and generates a diagnostic report.
func (d *Doctor) Run(ctx context.Context) *Report {
	report := &Report{
		Timestamp: time.Now().UTC(),
	}

	checks := []func(context.Context) CheckResult{
		d.checkConfig,
		d.checkProfile,
		d.checkDatabase,
		d.checkSoul,
		d.checkSkills,
		d.checkPolicy,
		d.checkLLM,
		d.checkEmbeddings,
		d.checkMCP,
		d.checkTools,
		d.checkWebTools,
		d.checkSubagents,
	}

	for _, check := range checks {
		res := check(ctx)
		report.Results = append(report.Results, res)
		report.Summary.Total++
		switch res.Status {
		case StatusPass:
			report.Summary.Passed++
		case StatusWarn:
			report.Summary.Warnings++
		case StatusFail:
			report.Summary.Failed++
		}
	}

	return report
}

func (d *Doctor) checkConfig(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "config",
		Title: "Configuration & Environment",
	}

	home := d.agisHome
	if home == "" {
		res.Status = StatusFail
		res.Message = "Unable to determine AGIS_HOME directory"
		res.Duration = time.Since(start)
		return res
	}

	cfgPath := filepath.Join(home, "config.yaml")
	info, err := os.Stat(cfgPath)
	if os.IsNotExist(err) {
		res.Status = StatusPass
		res.Message = "Using default built-in configuration (no custom config.yaml)"
		res.Details = append(res.Details, fmt.Sprintf("AGIS_HOME: %s", home))
		res.Duration = time.Since(start)
		return res
	} else if err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Access error on %s: %v", cfgPath, err)
		res.Duration = time.Since(start)
		return res
	}

	mode := info.Mode().Perm()
	if mode > 0o600 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("Config file mode %04o is looser than recommended 0600", mode)
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", cfgPath))
	} else {
		res.Status = StatusPass
		res.Message = fmt.Sprintf("Valid configuration file (%04o)", mode)
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", cfgPath))
	}

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkDatabase(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "database",
		Title: "SQLite & Persistent Memory",
	}

	dbPath := d.cfg.DB.Path
	if dbPath == "" {
		dbPath = filepath.Join(d.agisHome, "agis.db")
	}

	// Verify parent directory exists or can be created
	parent := filepath.Dir(dbPath)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Cannot create database directory %s: %v", parent, err)
		res.Duration = time.Since(start)
		return res
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Failed to open database %s: %v", dbPath, err)
		res.Duration = time.Since(start)
		return res
	}
	defer db.Close()

	// Ping database
	if err := db.PingContext(ctx); err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Database ping failed on %s: %v", dbPath, err)
		res.Duration = time.Since(start)
		return res
	}

	// Check integrity
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Database integrity check failed: %s (err: %v)", integrity, err)
		res.Duration = time.Since(start)
		return res
	}

	// Check schema migration version
	var userVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Failed to read schema version: %v", err)
		res.Duration = time.Since(start)
		return res
	}

	res.Details = append(res.Details, fmt.Sprintf("Location: %s", dbPath))
	res.Details = append(res.Details, fmt.Sprintf("Schema version: %d (latest: 7)", userVersion))

	// Collect row counts
	var convCount, msgCount, obsCount int
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM conversations").Scan(&convCount)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM messages").Scan(&msgCount)
	_ = db.QueryRowContext(ctx, "SELECT count(*) FROM observations").Scan(&obsCount)

	res.Details = append(res.Details, fmt.Sprintf("Records: %d conversations, %d messages, %d observations", convCount, msgCount, obsCount))

	if userVersion < 7 {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("Database schema version is %d (pending migrations exist, will auto-migrate on start)", userVersion)
	} else {
		res.Status = StatusPass
		res.Message = "Database is healthy and fully migrated"
	}

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkSoul(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "soul",
		Title: "Agent Identity (SOUL.md)",
	}

	soulPath := persona.SoulPath(d.agisHome)
	info, err := os.Stat(soulPath)
	if os.IsNotExist(err) {
		res.Status = StatusWarn
		res.Message = "SOUL.md not found (will be automatically seeded on first interactive run)"
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", soulPath))
	} else if err != nil {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Failed to read SOUL.md: %v", err)
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", soulPath))
	} else {
		res.Status = StatusPass
		res.Message = fmt.Sprintf("Durable identity file present (%d bytes)", info.Size())
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", soulPath))
	}

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkSkills(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "skills",
		Title: "Skill Hub & Registry",
	}

	skillsDir := filepath.Join(d.agisHome, "skills")
	loadedSkills, err := skills.LoadDir(skillsDir, nil)
	if err != nil {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("Skill directory warning: %v", err)
		res.Duration = time.Since(start)
		return res
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("%d skills loaded and validated", len(loadedSkills))
	res.Details = append(res.Details, fmt.Sprintf("Directory: %s", skillsDir))

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkPolicy(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "policy",
		Title: "Policy Guard & Permissions",
	}

	policyPath := filepath.Join(d.agisHome, "policy.yaml")
	store, err := policy.Load(policyPath)
	if err != nil || store.Broken() {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("Policy store is corrupted or unreadable: %v", err)
		res.Details = append(res.Details, fmt.Sprintf("Path: %s", policyPath))
		res.Duration = time.Since(start)
		return res
	}

	localTier, _ := store.Tier(ctx, "local")
	dockerTier, _ := store.Tier(ctx, "docker")
	sshTier, _ := store.Tier(ctx, "ssh")
	rules, _ := store.Rules(ctx)

	res.Status = StatusPass
	res.Message = fmt.Sprintf("Policy Guard active (local: %s, docker: %s, ssh: %s, rules: %d)", localTier, dockerTier, sshTier, len(rules))
	res.Details = append(res.Details, fmt.Sprintf("Store: %s", policyPath))

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkLLM(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "llm",
		Title: "LLM Provider Connectivity",
	}

	primaryProvider := strings.ToLower(d.cfg.LLM.Provider)
	if primaryProvider == "" {
		primaryProvider = "ollama"
	}
	primaryModel := d.cfg.LLM.Model
	if primaryModel == "" {
		primaryModel = "llama3.2"
	}

	primaryKeyCount := len(d.cfg.LLM.APIKeys)
	if d.cfg.LLM.APIKey != "" {
		primaryKeyCount++
	}
	if primaryKeyCount == 0 && len(d.cfg.LLM.APIKeys) > 0 {
		primaryKeyCount = len(d.cfg.LLM.APIKeys)
	}

	res.Details = append(res.Details, fmt.Sprintf("Primary Provider: %s", primaryProvider))
	res.Details = append(res.Details, fmt.Sprintf("Primary Model: %s", primaryModel))
	if primaryKeyCount > 0 {
		res.Details = append(res.Details, fmt.Sprintf("Primary Keys Configured: %d", primaryKeyCount))
	}

	pStatus, pMsg, pDetails := d.probeSingleLLM(ctx, primaryProvider, primaryModel, d.cfg.LLM.APIKey, d.cfg.LLM.APIKeys, d.cfg.LLM.BaseURL)
	res.Details = append(res.Details, pDetails...)

	// Probe configured fallbacks
	fallbackPassCount := 0
	for i, fb := range d.cfg.LLM.Fallbacks {
		fbProvider := strings.ToLower(fb.Provider)
		if fbProvider == "" {
			fbProvider = "ollama"
		}
		fbModel := fb.Model
		if fbModel == "" {
			fbModel = "llama3.2"
		}

		fbStatus, fbMsg, fbDetails := d.probeSingleLLM(ctx, fbProvider, fbModel, fb.APIKey, fb.APIKeys, fb.BaseURL)
		res.Details = append(res.Details, fmt.Sprintf("Fallback #%d (%s/%s): %s [%s]", i+1, fbProvider, fbModel, fbMsg, fbStatus))
		res.Details = append(res.Details, fbDetails...)
		if fbStatus == StatusPass {
			fallbackPassCount++
		}
	}

	if pStatus == StatusPass {
		res.Status = StatusPass
		res.Message = pMsg
	} else if fallbackPassCount > 0 {
		res.Status = StatusWarn
		res.Message = "Primary provider failed, but fallback provider(s) are operational"
	} else if len(d.cfg.LLM.Fallbacks) > 0 {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("All LLM providers failed: %s", pMsg)
	} else {
		res.Status = pStatus
		res.Message = pMsg
	}

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) probeSingleLLM(ctx context.Context, provider, model, apiKey string, apiKeys []string, customBaseURL string) (CheckStatus, string, []string) {
	var details []string

	switch provider {
	case "ollama":
		ollamaBase := customBaseURL
		if ollamaBase == "" {
			ollamaBase = d.ollamaURL
		}
		if ollamaBase == "" {
			ollamaBase = os.Getenv("OLLAMA_HOST")
		}
		if ollamaBase == "" {
			ollamaBase = "http://localhost:11434"
		}
		if !strings.HasPrefix(ollamaBase, "http://") && !strings.HasPrefix(ollamaBase, "https://") {
			ollamaBase = "http://" + ollamaBase
		}
		endpoint := strings.TrimRight(ollamaBase, "/") + "/api/tags"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return StatusFail, fmt.Sprintf("Creating request failed: %v", err), details
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			details = append(details, "Hint: make sure Ollama is running (`ollama serve`)")
			return StatusFail, fmt.Sprintf("Ollama is not reachable at %s: %v", endpoint, err), details
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return StatusFail, fmt.Sprintf("Ollama returned HTTP %d", resp.StatusCode), details
		}

		var tagsResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err == nil {
			found := false
			var modelNames []string
			for _, m := range tagsResp.Models {
				modelNames = append(modelNames, m.Name)
				if strings.HasPrefix(m.Name, model) || strings.HasPrefix(m.Name, model+":") {
					found = true
				}
			}
			if found {
				return StatusPass, fmt.Sprintf("Ollama is reachable and model %q is installed", model), details
			}
			details = append(details, fmt.Sprintf("Installed models: %s", strings.Join(modelNames, ", ")))
			details = append(details, fmt.Sprintf("Hint: run `ollama pull %s`", model))
			return StatusWarn, fmt.Sprintf("Ollama is reachable, but model %q was not found in installed models", model), details
		}
		return StatusPass, "Ollama is reachable", details

	case "openai", "openrouter":
		effectiveKey := apiKey
		if effectiveKey == "" && len(apiKeys) > 0 {
			effectiveKey = apiKeys[0]
		}
		if effectiveKey == "" {
			if provider == "openai" {
				effectiveKey = os.Getenv("OPENAI_API_KEY")
			} else {
				effectiveKey = os.Getenv("OPENROUTER_API_KEY")
			}
		}

		if effectiveKey == "" {
			details = append(details, "Hint: set `api_key` in config.yaml or export OPENAI_API_KEY / OPENROUTER_API_KEY")
			return StatusFail, fmt.Sprintf("Missing API key for provider %q", provider), details
		}

		baseURL := customBaseURL
		if baseURL == "" {
			baseURL = d.openAIBaseURL
		}
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		if baseURL == "" {
			if provider == "openrouter" {
				baseURL = "https://openrouter.ai/api/v1/models"
			} else {
				baseURL = "https://api.openai.com/v1/models"
			}
		} else {
			baseURL = strings.TrimRight(baseURL, "/") + "/models"
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
		if err != nil {
			return StatusPass, fmt.Sprintf("API key configured for %s", provider), details
		}
		req.Header.Set("Authorization", "Bearer "+effectiveKey)
		resp, err := d.httpClient.Do(req)
		if err != nil {
			return StatusWarn, fmt.Sprintf("API key configured, but endpoint verification timed out / network error: %v", err), details
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return StatusPass, fmt.Sprintf("%s API is reachable and API key is valid", strings.ToUpper(provider)), details
		} else if resp.StatusCode == http.StatusUnauthorized {
			return StatusFail, fmt.Sprintf("%s API key was rejected (HTTP 401 Unauthorized)", strings.ToUpper(provider)), details
		}
		return StatusWarn, fmt.Sprintf("%s endpoint returned HTTP %d", strings.ToUpper(provider), resp.StatusCode), details

	default:
		return StatusPass, fmt.Sprintf("Custom provider %q configured", provider), details
	}
}

func (d *Doctor) checkEmbeddings(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "embeddings",
		Title: "Vector Embeddings & Hybrid Search",
	}

	if !d.cfg.Embeddings.Enabled {
		res.Status = StatusPass
		res.Message = "Embeddings disabled (using pure FTS5 lexical search)"
		res.Duration = time.Since(start)
		return res
	}

	provider := d.cfg.Embeddings.Provider
	if provider == "" {
		provider = "ollama"
	}
	model := d.cfg.Embeddings.Model
	if model == "" {
		if provider == "ollama" {
			model = "nomic-embed-text"
		} else {
			model = "text-embedding-3-small"
		}
	}

	res.Status = StatusPass
	res.Message = fmt.Sprintf("Hybrid search enabled (provider: %s, model: %s, dimensions: %d)", provider, model, d.cfg.Embeddings.Dimensions)
	res.Details = append(res.Details, fmt.Sprintf("Batch size: %d", d.cfg.Embeddings.BatchSize))

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkMCP(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "mcp",
		Title: "Model Context Protocol (MCP) Servers",
	}

	if !d.cfg.MCP.Enabled {
		res.Status = StatusPass
		res.Message = "MCP subsystem disabled (no external servers)"
		res.Duration = time.Since(start)
		return res
	}

	if len(d.cfg.MCP.Servers) == 0 {
		res.Status = StatusPass
		res.Message = "MCP enabled with 0 servers configured"
		res.Duration = time.Since(start)
		return res
	}

	var failures []string
	var activeCount int

	for name, srv := range d.cfg.MCP.Servers {
		if srv.Disabled {
			res.Details = append(res.Details, fmt.Sprintf("Server %q: DISABLED", name))
			continue
		}
		activeCount++
		if srv.Command != "" {
			// Stdio transport: verify binary existence in PATH
			path, err := exec.LookPath(srv.Command)
			if err != nil {
				failures = append(failures, fmt.Sprintf("Server %q binary %q not found in PATH", name, srv.Command))
				res.Details = append(res.Details, fmt.Sprintf("Server %q: stdio binary %q NOT FOUND", name, srv.Command))
			} else {
				res.Details = append(res.Details, fmt.Sprintf("Server %q: stdio (%s)", name, path))
			}
		} else if srv.URL != "" {
			// SSE transport: verify endpoint URL syntax
			res.Details = append(res.Details, fmt.Sprintf("Server %q: SSE endpoint (%s)", name, srv.URL))
		} else {
			failures = append(failures, fmt.Sprintf("Server %q has neither command nor URL configured", name))
		}
	}

	if len(failures) > 0 {
		res.Status = StatusFail
		res.Message = fmt.Sprintf("%d MCP server configuration issues detected", len(failures))
	} else {
		res.Status = StatusPass
		res.Message = fmt.Sprintf("%d active MCP servers configured and valid", activeCount)
	}

	res.Duration = time.Since(start)
	return res
}

func (d *Doctor) checkTools(ctx context.Context) CheckResult {
	start := time.Now()
	res := CheckResult{
		Name:  "tools",
		Title: "Execution Backends & System Tools",
	}

	// 1. Local Shell
	shPath, err := exec.LookPath("sh")
	if err != nil {
		res.Status = StatusFail
		res.Message = "Local shell ('sh') was not found in PATH"
		res.Duration = time.Since(start)
		return res
	}
	res.Details = append(res.Details, fmt.Sprintf("Local shell: %s", shPath))

	// 2. Docker CLI (optional/warn)
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		res.Details = append(res.Details, "Docker CLI: not found in PATH (docker tool backend unavailable)")
	} else {
		res.Details = append(res.Details, fmt.Sprintf("Docker CLI: %s", dockerPath))
	}

	// 3. SSH CLI (optional/warn)
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		res.Details = append(res.Details, "SSH client: not found in PATH (ssh tool backend unavailable)")
	} else {
		res.Details = append(res.Details, fmt.Sprintf("SSH client: %s", sshPath))
	}

	res.Status = StatusPass
	res.Message = "Core execution tools available"
	res.Duration = time.Since(start)
	return res
}
