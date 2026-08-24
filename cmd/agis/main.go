// Command agis is the Autonomous Go Intelligent System entrypoint.
//
// It loads configuration, wires the SQLite repository, the LLM provider, the
// Brain loop, and the Bubbletea TUI together, then runs the interactive loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/adapters/tui"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
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

	ctx := context.Background()

	repo, err := memory.NewRepository(ctx, cfg.DB.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agis: %v\n", err)
		os.Exit(1)
	}
	defer repo.Close()

	provider := llm.NewProvider(cfg.LLM)

	// The TUI owns the token stream: the brain's sink writes here and the
	// model drains it to paint tokens in real time. Buffered so a slow update
	// loop back-pressures the provider instead of dropping tokens.
	stream := make(chan string, 64)

	brainOpts := []core.Option{core.WithSink(func(text string) {
		stream <- text
	})}
	if cfg.Memory.LearningEnabled {
		curator := memory.NewCurator(provider, repo, nil)
		summarizer := memory.NewSummarizer(provider, repo, nil)
		brainOpts = append(brainOpts,
			core.WithNudger(curator),
			core.WithSessionCloser(summarizer),
			core.WithRecallLimit(cfg.Memory.RecallLimit),
			core.WithNudgeEvery(cfg.Memory.NudgeEvery),
		)
	}
	brain := core.NewBrain(repo, provider, brainOpts...)

	app := tui.New(brain, repo, stream, tui.WithCloseTimeout(cfg.Memory.CloseTimeout))

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "agis: %v\n", err)
		os.Exit(1)
	}
}
