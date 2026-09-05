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
		fmt.Fprintf(stderr, "agis gateway: opening database: %v\n", err)
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

	var transcriber core.Transcriber
	if cfg.Multimodal.Audio.Enabled {
		if cfg.Multimodal.Audio.Provider == "openai" {
			whisperModel := cfg.Multimodal.Audio.Model
			if whisperModel == "" {
				whisperModel = "whisper-1"
			}
			transcriber = llm.NewWhisper(
				"",
				cfg.LLM.APIKey,
				whisperModel,
			)
			logger.Info("multimodal: audio transcriber initialized", "provider", "openai", "model", whisperModel)
		}
	}

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
		gateway.WithMultiplexerSessionManager(sessionManager),
		gateway.WithMultiplexerLogger(logger),
	)

	if cfg.Gateway.Telegram.Enabled {
		var tgOpts []gateway.TelegramOption
		tgOpts = append(tgOpts,
			gateway.WithTelegramHandler(mux.HandleEvent),
			gateway.WithTelegramLogger(logger),
		)
		if transcriber != nil {
			tgOpts = append(tgOpts, gateway.WithTelegramTranscriber(transcriber))
		}
		if cfg.Multimodal.Vision.MaxImageSizeMB > 0 {
			tgOpts = append(tgOpts, gateway.WithTelegramMaxImageSize(int64(cfg.Multimodal.Vision.MaxImageSizeMB)*1024*1024))
		}
		if cfg.Multimodal.Audio.MaxAudioSizeMB > 0 {
			tgOpts = append(tgOpts, gateway.WithTelegramMaxAudioSize(int64(cfg.Multimodal.Audio.MaxAudioSizeMB)*1024*1024))
		}

		tg := gateway.NewTelegramAdapter(
			cfg.Gateway.Telegram,
			tgOpts...,
		)
		mux.RegisterAdapter(tg)
	}

	if cfg.Gateway.Discord.Enabled {
		var dcOpts []gateway.DiscordOption
		dcOpts = append(dcOpts,
			gateway.WithDiscordHandler(mux.HandleEvent),
			gateway.WithDiscordLogger(logger),
		)
		if transcriber != nil {
			dcOpts = append(dcOpts, gateway.WithDiscordTranscriber(transcriber))
		}
		if cfg.Multimodal.Vision.MaxImageSizeMB > 0 {
			dcOpts = append(dcOpts, gateway.WithDiscordMaxImageSize(int64(cfg.Multimodal.Vision.MaxImageSizeMB)*1024*1024))
		}
		if cfg.Multimodal.Audio.MaxAudioSizeMB > 0 {
			dcOpts = append(dcOpts, gateway.WithDiscordMaxAudioSize(int64(cfg.Multimodal.Audio.MaxAudioSizeMB)*1024*1024))
		}

		dc := gateway.NewDiscordAdapter(
			cfg.Gateway.Discord,
			dcOpts...,
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
