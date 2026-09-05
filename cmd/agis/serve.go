package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/persona"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/server"
	"github.com/SalvucciFacundo/agis/internal/skills"
	"github.com/SalvucciFacundo/agis/internal/tools"
	"github.com/SalvucciFacundo/agis/internal/version"
)

// RunServeCLI runs the `agis serve` / `agis api` subcommand router.
func RunServeCLI(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runServeWithContext(ctx, args, stdout, stderr)
}

func runServeWithContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")
	profileFlag := fs.String("profile", "", "active configuration profile name")
	hostFlag := fs.String("host", "", "HTTP host to bind (overrides config)")
	portFlag := fs.Int("port", -1, "HTTP port to bind (overrides config)")
	apiKeyFlag := fs.String("api-key", "", "Bearer token for API authentication (overrides config)")
	corsFlag := fs.String("cors", "", "comma-separated allowed CORS origins (overrides config)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis serve [flags]\n       agis api [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

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
		fmt.Fprintf(stderr, "agis serve: %v\n", err)
		return 2
	}

	if *profileFlag != "" {
		if err := config.SetActiveProfile(*profileFlag); err != nil {
			fmt.Fprintf(stderr, "agis serve: invalid profile %q: %v\n", *profileFlag, err)
			return 2
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis serve: loading config: %v\n", err)
		return 1
	}

	if *hostFlag != "" {
		cfg.Server.Host = *hostFlag
	}
	if *portFlag >= 0 {
		cfg.Server.Port = *portFlag
	}
	if *apiKeyFlag != "" {
		cfg.Server.APIKey = *apiKeyFlag
	}
	if *corsFlag != "" {
		parts := strings.Split(*corsFlag, ",")
		var origins []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		cfg.Server.CORSOrigins = origins
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
		fmt.Fprintf(stderr, "agis serve: opening database: %v\n", err)
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

	if cfg.Agent.EvolutionEnabled {
		evolution := persona.NewEvolution(repo, logger)
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

	if cfg.Memory.LearningEnabled {
		memoryProvider := llm.NewProviderForTask(cfg.LLM, cfg.Memory.Provider, cfg.Memory.Model)
		curator := memory.NewCurator(memoryProvider, repo, nil)
		summarizer := memory.NewSummarizer(memoryProvider, repo, nil)
		creator := skills.NewCreator(provider, repo, cfg.Skills.Enabled, nil)
		brainOpts = append(brainOpts,
			core.WithNudger(curator),
			core.WithSessionCloser(summarizer),
			core.WithSkillCreator(creator),
			core.WithRecallLimit(cfg.Memory.RecallLimit),
			core.WithNudgeEvery(cfg.Memory.NudgeEvery),
		)
	}

	pstore, perr := policy.Load(filepath.Join(config.AgisHome(), "policy.yaml"))
	if perr != nil {
		logger.Warn("policy: loading (fail-closed)", "error", perr)
	}
	pstore.SetAuditSink(repo)

	// Non-interactive auto-deny approver for HTTP API background execution
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

	srvOpts := server.Options{
		Host:         cfg.Server.Host,
		Port:         cfg.Server.Port,
		APIKey:       cfg.Server.APIKey,
		CORSOrigins:  cfg.Server.CORSOrigins,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		Brain:        brain,
		Logger:       logger,
		Profile:      config.ActiveProfile(),
		Version:      version.Get().Version,
		Provider:     cfg.LLM.Provider,
		Model:        cfg.LLM.Model,
	}

	srv := server.New(srvOpts)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	fmt.Fprintf(stdout, "AGIS REST API Server listening on %s (profile: %s, model: %s/%s)\n",
		srv.Addr(), config.ActiveProfile(), cfg.LLM.Provider, cfg.LLM.Model)

	select {
	case <-ctx.Done():
		fmt.Fprintf(stdout, "Shutting down API Server gracefully...\n")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(stderr, "agis serve: shutdown error: %v\n", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "agis serve: server error: %v\n", err)
			return 1
		}
		return 0
	}
}
