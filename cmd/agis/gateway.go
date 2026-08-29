package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/persona"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/session"
	"github.com/SalvucciFacundo/agis/internal/skills"
	"github.com/SalvucciFacundo/agis/internal/tools"
)

// RunGatewayCLI runs the `agis gateway` subcommand daemon.
func RunGatewayCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runGatewayWithContext(ctx, args, stdout, stderr)
}

func runGatewayWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis gateway [run] [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	// Filter out optional "run" subcommand argument
	var flagArgs []string
	for _, a := range args {
		if a == "run" {
			continue
		}
		flagArgs = append(flagArgs, a)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis gateway: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis gateway: loading config: %v\n", err)
		return 1
	}

	if !cfg.Gateway.Enabled {
		fmt.Fprintf(stderr, "agis gateway: gateway is disabled in configuration\n")
		return 1
	}

	if !cfg.Gateway.Telegram.Enabled && !cfg.Gateway.Discord.Enabled {
		fmt.Fprintf(stderr, "agis gateway: no chat adapters (telegram, discord) enabled\n")
		return 1
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	repo, err := memory.NewRepository(ctx, cfg.DB.Path)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "agis gateway: opening database: %v\n", err)
		return 1
	}
	defer repo.Close()

	provider := llm.NewProvider(cfg.LLM)

	identity, err := persona.LoadSoul(persona.SoulPath(config.AgisHome()), logger)
	if err != nil {
		logger.Warn("persona: loading SOUL.md", "error", err)
	}

	brainOpts := []core.Option{
		core.WithIdentity(identity),
	}

	var evolution *persona.Evolution
	if cfg.Agent.EvolutionEnabled {
		evolution = persona.NewEvolution(repo, logger)
		brainOpts = append(brainOpts, core.WithEvolution(evolution))
	}
	if cfg.Skills.Enabled {
		hub := skills.NewHub(repo, logger)
		if err := hub.LoadDir(ctx, cfg.Skills.Dir); err != nil {
			logger.Warn("skills: loading directory", "error", err)
		}
		brainOpts = append(brainOpts, core.WithSkills(hub))
	}

	var summarizer *memory.Summarizer
	if cfg.Memory.LearningEnabled {
		curator := memory.NewCurator(provider, repo, nil)
		summarizer = memory.NewSummarizer(provider, repo, nil)
		creator := skills.NewCreator(provider, repo, cfg.Skills.Enabled, nil)
		brainOpts = append(brainOpts,
			core.WithNudger(curator),
			core.WithSessionCloser(summarizer),
			core.WithSkillCreator(creator),
			core.WithRecallLimit(cfg.Memory.RecallLimit),
			core.WithNudgeEvery(cfg.Memory.NudgeEvery),
		)
	}

	sessionManager := session.New(repo, summarizer, logger)

	pstore, perr := policy.Load(filepath.Join(config.AgisHome(), "policy.yaml"))
	if perr != nil {
		logger.Warn("policy: loading (fail-closed)", "error", perr)
	}
	pstore.SetAuditSink(repo)

	// Non-interactive auto-deny approver for gateway daemon
	autoApprover := gateway.NewAutoDenyApprover(logger)

	if cfg.Tools.Enabled {
		runners := tools.Select(cfg.Tools, logger)
		brainOpts = append(brainOpts, core.WithTools(
			runners,
			pstore,
			autoApprover,
		))
	}

	brain := core.NewBrain(repo, provider, brainOpts...)

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
		gateway.WithMultiplexerSessionManager(sessionManager),
		gateway.WithMultiplexerLogger(logger),
	)

	if cfg.Gateway.Telegram.Enabled {
		tg := gateway.NewTelegramAdapter(
			cfg.Gateway.Telegram,
			gateway.WithTelegramHandler(mux.HandleEvent),
			gateway.WithTelegramLogger(logger),
		)
		mux.RegisterAdapter(tg)
	}

	if cfg.Gateway.Discord.Enabled {
		dc := gateway.NewDiscordAdapter(
			cfg.Gateway.Discord,
			gateway.WithDiscordHandler(mux.HandleEvent),
			gateway.WithDiscordLogger(logger),
		)
		mux.RegisterAdapter(dc)
	}

	if err := mux.Start(ctx); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "agis gateway: starting multiplexer: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "agis gateway: multiplexer daemon running. Press Ctrl+C to terminate.\n")

	<-ctx.Done()

	fmt.Fprintf(stdout, "agis gateway: shutting down gracefully...\n")
	if err := mux.Stop(); err != nil {
		fmt.Fprintf(stderr, "agis gateway: stop error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "agis gateway: stopped successfully.\n")
	return 0
}
