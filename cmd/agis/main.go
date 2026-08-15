// Command agis is the Autonomous Go Intelligent System entrypoint.
//
// M1 skeleton (PR1): parse the -config flag, load configuration, and print a
// diagnostic. Repository, LLM adapters, and the Bubbletea TUI are wired in
// PRs 2-4.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/SalvucciFacundo/agis/internal/config"
)

func main() {
	fs := flag.NewFlagSet("agis", flag.ExitOnError)
	configPath := fs.String(
		"config",
		"",
		"path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)",
	)
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agis: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "agis: loaded config (provider=%s model=%s db=%s)\n",
		cfg.LLM.Provider, cfg.LLM.Model, cfg.DB.Path)
}
