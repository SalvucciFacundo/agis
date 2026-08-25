// Command agis is the Autonomous Go Intelligent System entrypoint.
//
// It loads configuration, wires the SQLite repository, the LLM provider, the
// Brain loop, and the Bubbletea TUI together, then runs the interactive loop.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/adapters/tui"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/persona"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/skills"
	"github.com/SalvucciFacundo/agis/internal/tools"
)

func main() {
	// Subcommands route before any flag parsing (design D9): the interactive
	// TUI is the default surface, everything else is a managed subcommand.
	if len(os.Args) > 1 && os.Args[1] == "policy" {
		os.Exit(policy.RunCLI(os.Args[2:], os.Stdout, os.Stderr))
	}

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

	identity, err := persona.LoadSoul(persona.SoulPath(config.AgisHome()), slog.Default())
	if err != nil {
		slog.Warn("persona: loading SOUL.md", "error", err)
	}

	brainOpts := []core.Option{
		core.WithSink(func(text string) { stream <- text }),
		core.WithIdentity(identity),
	}

	var evolution *persona.Evolution
	if cfg.Agent.EvolutionEnabled {
		evolution = persona.NewEvolution(repo, slog.Default())
		brainOpts = append(brainOpts, core.WithEvolution(evolution))
	}
	if cfg.Skills.Enabled {
		hub := skills.NewHub(repo, slog.Default())
		if err := hub.LoadDir(ctx, cfg.Skills.Dir); err != nil {
			slog.Warn("skills: loading directory", "error", err)
		}
		regDir := filepath.Join(config.AgisHome(), ".atl")
		if mkErr := os.MkdirAll(regDir, 0o700); mkErr == nil {
			hub.SyncRegistry(filepath.Join(regDir, "skill-registry.md"))
		} else {
			slog.Warn("skills: creating registry directory", "error", mkErr)
		}
		brainOpts = append(brainOpts, core.WithSkills(hub))
	}
	if cfg.Memory.LearningEnabled {
		curator := memory.NewCurator(provider, repo, nil)
		summarizer := memory.NewSummarizer(provider, repo, nil)
		creator := skills.NewCreator(provider, repo, cfg.Skills.Enabled, nil)
		brainOpts = append(brainOpts,
			core.WithNudger(curator),
			core.WithSessionCloser(summarizer),
			core.WithSkillCreator(creator),
			core.WithRecallLimit(cfg.Memory.RecallLimit),
			core.WithNudgeEvery(cfg.Memory.NudgeEvery),
		)
	}
	var approvalReq chan core.GuardRequest
	var approvalResp chan core.Scope
	approver := func(ctx context.Context, req core.GuardRequest) core.Scope {
		if approvalReq == nil {
			return core.ScopeDeny
		}
		select {
		case approvalReq <- req:
		case <-ctx.Done():
			return core.ScopeDeny
		}
		select {
		case sc := <-approvalResp:
			return sc
		case <-ctx.Done():
			return core.ScopeDeny
		}
	}

	if cfg.Tools.Enabled {
		pstore, perr := policy.Load(filepath.Join(config.AgisHome(), "policy.yaml"))
		if perr != nil {
			slog.Warn("policy: loading (fail-closed)", "error", perr)
		}
		pstore.SetAuditSink(repo)

		if approvalReq == nil {
			approvalReq = make(chan core.GuardRequest)
			approvalResp = make(chan core.Scope)
		}
		brainOpts = append(brainOpts, core.WithTools(
			tools.NewLocal(0),
			pstore,
			func(ctx context.Context, req core.GuardRequest) core.Scope {
				sc := approver(ctx, req)
				switch sc {
				case core.ScopeSession, core.ScopeAlways:
					if err := pstore.ResolveAsk(ctx, req, sc); err != nil {
						slog.Warn("policy: resolving ask", "error", err)
					}
				}
				return sc
			},
		))
	}

	brain := core.NewBrain(repo, provider, brainOpts...)

	tuiOpts := []tui.Option{
		tui.WithCloseTimeout(cfg.Memory.CloseTimeout),
		tui.WithOverlays(persona.NewOverlays(cfg.Agent.Personalities)),
	}
	if evolution != nil {
		tuiOpts = append(tuiOpts, tui.WithEvolution(evolution))
	}
	if approvalReq != nil {
		tuiOpts = append(tuiOpts,
			tui.WithApprovalChannels(approvalReq, approvalResp))
	}
	app := tui.New(brain, repo, stream, tuiOpts...)

	if _, err := tea.NewProgram(app).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "agis: %v\n", err)
		os.Exit(1)
	}
}
