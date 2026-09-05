package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/SalvucciFacundo/agis/internal/setup"
)

// RunSetupCLI routes the `agis setup` and `agis init` setup wizard subcommand.
func RunSetupCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help") {
		printSetupUsage(stdout)
		return 0
	}

	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)

	provider := fs.String("provider", "", "LLM provider (ollama, openai, openrouter, anthropic)")
	model := fs.String("model", "", "LLM model name")
	apiKey := fs.String("api-key", "", "API key or secret token for provider")
	baseURL := fs.String("base-url", "", "Custom base URL for provider endpoint")
	nonInteractive := fs.Bool("non-interactive", false, "Run in non-interactive headless mode")
	force := fs.Bool("force", false, "Overwrite existing config and skip connectivity failures")
	configPath := fs.String("config", "", "Custom destination path for config.yaml")
	profile := fs.String("profile", "", "Target profile name to configure")

	fs.Usage = func() {
		printSetupUsage(stdout)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printSetupUsage(stdout)
			return 0
		}
		return 2
	}

	wizard := setup.NewWizard(stdin, stdout, stderr)
	return wizard.Run(setup.SetupOptions{
		Provider:       *provider,
		Model:          *model,
		APIKey:         *apiKey,
		BaseURL:        *baseURL,
		NonInteractive: *nonInteractive,
		Force:          *force,
		ConfigPath:     *configPath,
		Profile:        *profile,
	})
}

func printSetupUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis setup [flags] (alias: agis init)\n\n")
	fmt.Fprintf(w, "Interactive setup wizard and headless initialization for AGIS.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -provider string         LLM provider (ollama, openai, openrouter, anthropic)\n")
	fmt.Fprintf(w, "  -model string            Model identifier (e.g. llama3.2, gpt-4o)\n")
	fmt.Fprintf(w, "  -api-key string          API secret key for provider\n")
	fmt.Fprintf(w, "  -base-url string         Custom endpoint base URL\n")
	fmt.Fprintf(w, "  -non-interactive         Bypass terminal prompt and run headlessly\n")
	fmt.Fprintf(w, "  -force                   Overwrite existing config without prompting\n")
	fmt.Fprintf(w, "  -config string           Custom configuration file path override\n")
	fmt.Fprintf(w, "  -profile string          Target profile name to configure\n")
	fmt.Fprintf(w, "  -h, --help               Show help\n")
}
