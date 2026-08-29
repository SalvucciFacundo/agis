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
	"github.com/SalvucciFacundo/agis/internal/webhook"
)

// RunWebhookCLI runs the `agis webhook` subcommand router.
func RunWebhookCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runWebhookWithContext(ctx, args, stdout, stderr)
}

func runWebhookWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("webhook", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")
	hostFlag := fs.String("host", "", "HTTP host to bind (overrides config)")
	portFlag := fs.Int("port", 0, "HTTP port to bind (overrides config)")
	pathFlag := fs.String("path", "", "HTTP endpoint path (overrides config)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis webhook [run] [flags]\n\n")
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
		fmt.Fprintf(stderr, "agis webhook: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis webhook: loading config: %v\n", err)
		return 1
	}

	if !cfg.Webhook.Enabled && *portFlag == 0 {
		fmt.Fprintf(stderr, "agis webhook: webhook listener is disabled in configuration\n")
		return 1
	}

	// Override flags
	if *hostFlag != "" {
		cfg.Webhook.Host = *hostFlag
	}
	if *portFlag != 0 {
		cfg.Webhook.Port = *portFlag
	}
	if *pathFlag != "" {
		cfg.Webhook.Path = *pathFlag
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))

	repo, err := memory.NewRepository(ctx, cfg.DB.Path)
	if err != nil {
		fmt.Fprintf(stderr, "agis webhook: opening database: %v\n", err)
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
		regDir := filepath.Join(config.AgisHome(), ".atl")
		if mkErr := os.MkdirAll(regDir, 0o700); mkErr == nil {
			hub.SyncRegistry(filepath.Join(regDir, "skill-registry.md"))
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

	// Non-interactive auto-deny approver for webhook background execution
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

	// Setup gateway multiplexer if gateway is enabled or targets configured
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
			fmt.Fprintf(stderr, "agis webhook: starting gateway multiplexer for targets: %v\n", err)
			return 1
		}
	}

	whConfig := webhook.Config{
		Host:             cfg.Webhook.Host,
		Port:             cfg.Webhook.Port,
		Path:             cfg.Webhook.Path,
		Secret:           cfg.Webhook.Secret,
		DefaultSessionID: cfg.Webhook.DefaultSessionID,
	}
	if cfg.Webhook.Target != nil {
		whConfig.Target = &webhook.Target{
			Adapter:   cfg.Webhook.Target.Adapter,
			Recipient: cfg.Webhook.Target.Recipient,
		}
	}

	serverOpts := []webhook.Option{
		webhook.WithBrain(brain),
		webhook.WithRepo(repo),
		webhook.WithLogger(logger),
		webhook.WithApprover(autoApprover),
	}
	if mux != nil {
		serverOpts = append(serverOpts, webhook.WithSender(mux))
	}

	server := webhook.NewServer(whConfig, serverOpts...)

	fmt.Fprintf(stdout, "Starting AGIS webhook listener on %s:%d%s...\n", cfg.Webhook.Host, cfg.Webhook.Port, cfg.Webhook.Path)
	if err := server.Start(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(stderr, "agis webhook: server stopped with error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "AGIS webhook server stopped.\n")
	return 0
}
