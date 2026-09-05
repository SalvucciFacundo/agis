package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// SetupOptions defines the parameters for the setup wizard.
type SetupOptions struct {
	Provider       string
	Model          string
	APIKey         string
	BaseURL        string
	NonInteractive bool
	Force          bool
	ConfigPath     string
	Profile        string
}

// Wizard coordinates user interaction and configuration persistence.
type Wizard struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// NewWizard creates a new Setup Wizard instance with designated I/O streams.
func NewWizard(stdin io.Reader, stdout, stderr io.Writer) *Wizard {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Wizard{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}

// Run executes either the non-interactive or interactive setup workflow.
// Returns POSIX exit codes: 0 for success, 1 for operational/runtime errors, 2 for syntax/usage errors.
func (w *Wizard) Run(opts SetupOptions) int {
	if opts.NonInteractive {
		return w.runNonInteractive(opts)
	}
	return w.runInteractive(opts)
}

func (w *Wizard) runNonInteractive(opts SetupOptions) int {
	provider := strings.ToLower(strings.TrimSpace(opts.Provider))
	if provider == "" {
		provider = "ollama"
	}

	// Validate provider
	switch provider {
	case "ollama", "openai", "openrouter", "anthropic":
		// valid
	default:
		fmt.Fprintf(w.stderr, "agis setup: invalid provider %q (supported: ollama, openai, openrouter, anthropic)\n", provider)
		return 2
	}

	// Validate required API keys for cloud providers
	if provider != "ollama" && strings.TrimSpace(opts.APIKey) == "" && !opts.Force {
		fmt.Fprintf(w.stderr, "agis setup: -api-key is required for provider %q\n", provider)
		return 2
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultModelForProvider(provider)
	}

	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURLForProvider(provider)
	}

	destPath := w.resolveConfigPath(opts)

	// Overwrite guard
	if _, err := os.Stat(destPath); err == nil && !opts.Force {
		fmt.Fprintf(w.stderr, "agis setup: config file %q already exists (use -force to overwrite)\n", destPath)
		return 1
	}

	// Connectivity probe
	if !opts.Force {
		ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
		defer cancel()

		if err := ProbeConnectivity(ctx, provider, baseURL, opts.APIKey); err != nil {
			fmt.Fprintf(w.stderr, "agis setup: connectivity probe failed: %v\n", err)
			return 1
		}
	}

	if err := w.saveConfiguration(destPath, provider, model, opts.APIKey, baseURL); err != nil {
		fmt.Fprintf(w.stderr, "agis setup: saving config: %v\n", err)
		return 1
	}

	fmt.Fprintf(w.stdout, "AGIS setup complete. Configuration saved to %s (mode 0600)\n", destPath)
	return 0
}

func (w *Wizard) runInteractive(opts SetupOptions) int {
	scanner := bufio.NewScanner(w.stdin)

	fmt.Fprintln(w.stdout, "=== AGIS Setup Wizard ===")
	fmt.Fprintln(w.stdout, "Configure your Language Model provider and credentials.")
	fmt.Fprintln(w.stdout)

	// Step 1: Provider selection
	provider := opts.Provider
	if provider == "" {
		fmt.Fprintln(w.stdout, "Select LLM Provider:")
		fmt.Fprintln(w.stdout, "  1) ollama      (Local LLM)")
		fmt.Fprintln(w.stdout, "  2) openai      (OpenAI GPT models)")
		fmt.Fprintln(w.stdout, "  3) openrouter  (OpenRouter multi-model)")
		fmt.Fprintln(w.stdout, "  4) anthropic   (Anthropic Claude)")
		fmt.Fprint(w.stdout, "Choice [default: ollama]: ")

		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			switch input {
			case "1", "ollama", "":
				provider = "ollama"
			case "2", "openai":
				provider = "openai"
			case "3", "openrouter":
				provider = "openrouter"
			case "4", "anthropic":
				provider = "anthropic"
			default:
				provider = strings.ToLower(input)
			}
		}
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "ollama"
	}

	// Step 2: API Key entry
	apiKey := opts.APIKey
	if provider != "ollama" && apiKey == "" {
		fmt.Fprintf(w.stdout, "Enter API Key for %s: ", provider)
		if scanner.Scan() {
			apiKey = strings.TrimSpace(scanner.Text())
		}
	}

	// Step 3: Model selection
	model := opts.Model
	defaultModel := defaultModelForProvider(provider)
	if model == "" {
		fmt.Fprintf(w.stdout, "Enter Model name [default: %s]: ", defaultModel)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				model = input
			} else {
				model = defaultModel
			}
		} else {
			model = defaultModel
		}
	}

	// Step 4: Base URL entry
	baseURL := opts.BaseURL
	defaultBaseURL := defaultBaseURLForProvider(provider)
	if baseURL == "" {
		fmt.Fprintf(w.stdout, "Enter Base URL [default: %s]: ", defaultBaseURL)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if input != "" {
				baseURL = input
			} else {
				baseURL = defaultBaseURL
			}
		} else {
			baseURL = defaultBaseURL
		}
	}

	// Step 5: Connectivity probe
	if !opts.Force {
		fmt.Fprintln(w.stdout, "Testing connectivity to provider...")
		ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
		defer cancel()

		if err := ProbeConnectivity(ctx, provider, baseURL, apiKey); err != nil {
			fmt.Fprintf(w.stderr, "Warning: Connectivity check failed: %v\n", err)
			fmt.Fprint(w.stdout, "Save configuration anyway? (y/N): ")
			if scanner.Scan() {
				ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(w.stdout, "Setup aborted.")
					return 1
				}
			} else {
				return 1
			}
		} else {
			fmt.Fprintln(w.stdout, "[PASS] Connectivity check succeeded!")
		}
	}

	// Step 6: Atomic persistence
	destPath := w.resolveConfigPath(opts)
	if err := w.saveConfiguration(destPath, provider, model, apiKey, baseURL); err != nil {
		fmt.Fprintf(w.stderr, "agis setup: error saving configuration: %v\n", err)
		return 1
	}

	fmt.Fprintf(w.stdout, "\nSetup completed successfully! Configuration saved to %s (mode 0600)\n", destPath)
	return 0
}

func (w *Wizard) resolveConfigPath(opts SetupOptions) string {
	if opts.ConfigPath != "" {
		return opts.ConfigPath
	}
	if opts.Profile != "" {
		return filepath.Join(config.ProfileDir(opts.Profile), "config.yaml")
	}
	return config.ResolvePath("")
}

func (w *Wizard) saveConfiguration(path, provider, model, apiKey, baseURL string) error {
	cfg, err := config.Load(path)
	if err != nil {
		cfg = &config.Config{}
	}

	cfg.LLM.Provider = provider
	cfg.LLM.Model = model
	cfg.LLM.APIKey = apiKey
	cfg.LLM.BaseURL = baseURL

	return config.Save(path, cfg)
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "ollama":
		return "llama3.2"
	case "openai":
		return "gpt-4o"
	case "openrouter":
		return "anthropic/claude-3.5-sonnet"
	case "anthropic":
		return "claude-3-5-sonnet-20241022"
	default:
		return "llama3.2"
	}
}

func defaultBaseURLForProvider(provider string) string {
	switch provider {
	case "ollama":
		return defaultOllamaBaseURL
	case "openai":
		return defaultOpenAIBaseURL
	case "openrouter":
		return defaultOpenRouterBaseURL
	case "anthropic":
		return defaultAnthropicBaseURL
	default:
		return ""
	}
}
