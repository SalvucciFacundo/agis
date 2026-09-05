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
	"github.com/SalvucciFacundo/agis/internal/cron"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/persona"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/session"
	"github.com/SalvucciFacundo/agis/internal/skills"
	"github.com/SalvucciFacundo/agis/internal/tools"
)

// RunCronCLI runs the `agis cron` subcommand router.
func RunCronCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runCronWithContext(ctx, args, stdout, stderr)
}

func runCronWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	subcommand := "run"
	var flagArgs []string

	if len(args) > 0 {
		switch args[0] {
		case "run", "list":
			subcommand = args[0]
			flagArgs = args[1:]
		case "-h", "--help", "-help":
			printCronUsage(stdout)
			return 0
		default:
			flagArgs = args
		}
	}

	fs := flag.NewFlagSet("cron "+subcommand, flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")

	fs.Usage = func() {
		printCronUsage(stdout)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis cron: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis cron: loading config: %v\n", err)
		return 1
	}

	switch subcommand {
	case "list":
		return runCronList(cfg, stdout)
	case "run":
		return runCronDaemon(ctx, cfg, stdout, stderr)
	default:
		printCronUsage(stdout)
		return 2
	}
}

func printCronUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis cron [run|list] [flags]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  run     Run the background cron scheduler daemon (default)\n")
	fmt.Fprintf(w, "  list    List all configured cron jobs with schedule and targets\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -config string\n")
	fmt.Fprintf(w, "        path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)\n")
}

func runCronList(cfg *config.Config, stdout io.Writer) int {
	if len(cfg.Cron.Jobs) == 0 {
		fmt.Fprintf(stdout, "No cron jobs configured.\n")
		return 0
	}

	fmt.Fprintf(stdout, "Configured Cron Jobs (%d):\n", len(cfg.Cron.Jobs))
	for _, j := range cfg.Cron.Jobs {
		targetStr := "(none)"
		if j.Target != nil && j.Target.Adapter != "" {
			targetStr = fmt.Sprintf("%s -> %s", j.Target.Adapter, j.Target.Recipient)
		}
		sessionStr := j.SessionID
		if sessionStr == "" {
			sessionStr = fmt.Sprintf("cron:%s (ephemeral)", j.Name)
		}

		fmt.Fprintf(stdout, "- Name:     %s\n", j.Name)
		fmt.Fprintf(stdout, "  Schedule: %s\n", j.Schedule)
		fmt.Fprintf(stdout, "  Prompt:   %s\n", j.Prompt)
		fmt.Fprintf(stdout, "  Session:  %s\n", sessionStr)
		fmt.Fprintf(stdout, "  Target:   %s\n\n", targetStr)
	}

	return 0
}

func runCronDaemon(ctx context.Context, cfg *config.Config, stdout, stderr io.Writer) int {
	if !cfg.Cron.Enabled {
		fmt.Fprintf(stderr, "agis cron: cron scheduler is disabled in configuration\n")
		return 1
	}

	if len(cfg.Cron.Jobs) == 0 {
		fmt.Fprintf(stderr, "agis cron: no cron jobs configured\n")
		return 1
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	var repoOpts []memory.Option
	if cfg.Embeddings.Enabled {
		embedder, err := llm.NewEmbedder(cfg.Embeddings, cfg.LLM.APIKey)
		if err != nil {
			logger.Warn("embeddings: initializing embedder (falling back to FTS5)", "error", err)
		} else {
			repoOpts = append(repoOpts, memory.WithEmbedder(embedder))
		}
	}

	repo, err := memory.NewRepository(ctx, cfg.DB.Path, repoOpts...)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(stderr, "agis cron: opening database: %v\n", err)
		return 1
	}
	defer repo.Close()

	provider := llm.NewResilientProvider(cfg.LLM)

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
		memoryProvider := llm.NewProviderForTask(cfg.LLM, cfg.Memory.Provider, cfg.Memory.Model)
		curator := memory.NewCurator(memoryProvider, repo, nil)
		summarizer = memory.NewSummarizer(memoryProvider, repo, nil)
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

	// Non-interactive auto-deny approver for cron background execution
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

	var mux *gateway.Multiplexer
	if cfg.Gateway.Enabled {
		mux = gateway.NewMultiplexer(
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
			fmt.Fprintf(stderr, "agis cron: starting gateway multiplexer for targets: %v\n", err)
			return 1
		}
	}

	engineOpts := []cron.EngineOption{
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
		cron.WithEngineLogger(logger),
	}
	if mux != nil {
		engineOpts = append(engineOpts, cron.WithEngineSender(mux))
	}

	engine := cron.NewEngine(engineOpts...)

	for _, j := range cfg.Cron.Jobs {
		var target *cron.Target
		if j.Target != nil {
			target = &cron.Target{
				Adapter:   j.Target.Adapter,
				Recipient: j.Target.Recipient,
			}
		}

		job := cron.Job{
			Name:      j.Name,
			Schedule:  j.Schedule,
			Prompt:    j.Prompt,
			SessionID: j.SessionID,
			Target:    target,
		}
		if err := engine.AddJob(job); err != nil {
			fmt.Fprintf(stderr, "agis cron: invalid job %q: %v\n", j.Name, err)
			if mux != nil {
				_ = mux.Stop()
			}
			return 1
		}
	}

	if err := engine.Start(ctx); err != nil {
		if ctx.Err() != nil {
			if mux != nil {
				_ = mux.Stop()
			}
			return 0
		}
		fmt.Fprintf(stderr, "agis cron: starting engine: %v\n", err)
		if mux != nil {
			_ = mux.Stop()
		}
		return 1
	}

	fmt.Fprintf(stdout, "agis cron: scheduler daemon running with %d jobs. Press Ctrl+C to terminate.\n", len(cfg.Cron.Jobs))

	<-ctx.Done()

	fmt.Fprintf(stdout, "agis cron: shutting down gracefully...\n")
	if err := engine.Stop(); err != nil {
		fmt.Fprintf(stderr, "agis cron: engine stop error: %v\n", err)
	}

	if mux != nil {
		if err := mux.Stop(); err != nil {
			fmt.Fprintf(stderr, "agis cron: gateway multiplexer stop error: %v\n", err)
		}
	}

	fmt.Fprintf(stdout, "agis cron: stopped successfully.\n")
	return 0
}
