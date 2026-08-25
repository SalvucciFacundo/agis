package policy

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
)

// backends is the canonical backend list used when a command does not name
// one explicitly.
var backends = []string{"local", "docker", "ssh"}

// policyPath resolves $AGIS_HOME/policy.yaml.
func policyPath() string {
	return filepath.Join(config.AgisHome(), "policy.yaml")
}

// RunCLI executes `agis policy <sub>` and returns the process exit code.
// It is routed from main before any other flag parsing (design D9).
func RunCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 1
	}

	ctx := context.Background()

	// The audit sink needs the repository; every mutating command opens it.
	store, closeRepo, err := openAuditedStore(ctx, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "agis policy: %v\n", err)
		return 1
	}
	defer closeRepo()

	switch args[0] {
	case "init":
		return cmdInit(ctx, store, args[1:], stdout, stderr)
	case "set":
		return cmdSet(ctx, store, args[1:], stdout, stderr)
	case "rm":
		return cmdRm(ctx, store, args[1:], stdout, stderr)
	case "show":
		return cmdShow(ctx, store, args[1:], stdout, stderr)
	case "tier":
		return cmdTier(ctx, store, args[1:], stdout, stderr)
	case "test":
		return cmdTest(ctx, store, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agis policy: unknown subcommand %q\n\n", args[0])
		usage(stderr)
		return 1
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: agis policy <subcommand> [flags]

  init [--force]                                   create policy.yaml with sandbox defaults
  set [-b backend] <category> <pattern> <action>   allow|deny a subject pattern
  rm [-b backend] <category> <pattern>             remove rules matching exactly
  show                                             print tiers and rules
  tier <backend> <sandbox|standard>                persist a baseline posture
  test [-c category] [-b backend] <subject...>     dry-run: print the decision
`)
}

// openAuditedStore loads the policy store with the repository wired as audit
// sink. The returned func closes the repository.
func openAuditedStore(ctx context.Context, stderr io.Writer) (*Store, func(), error) {
	repo, err := memory.NewRepository(ctx, filepath.Join(config.AgisHome(), "agis.db"))
	if err != nil {
		return nil, nil, fmt.Errorf("opening repository for audit: %w", err)
	}
	store, loadErr := Load(policyPath())
	store.SetAuditSink(repo)

	// A read failure (not a missing file) is surfaced immediately; parse
	// corruption keeps the store in deny-all mode and each subcommand reports
	// it through store.Broken().
	if loadErr != nil && !os.IsNotExist(loadErr) {
		fmt.Fprintf(stderr, "agis policy: %v\n", loadErr)
	}
	closeFn := func() { _ = repo.Close() }
	return store, closeFn, nil
}

// cmdInit creates policy.yaml with safe defaults. It refuses to overwrite an
// existing file unless --force is given (POL-004).
func cmdInit(ctx context.Context, store *Store, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite an existing policy.yaml")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	path := policyPath()
	if _, err := os.Stat(path); err == nil && !*force {
		fmt.Fprintf(stderr, "agis policy: %s already exists (use --force to overwrite)\n", path)
		return 1
	}

	// A fresh default: every backend at sandbox, no rules yet.
	fresh := &Store{
		path:   path,
		file:   policyFile{Tiers: map[string]string{}, Rules: map[string][]ruleYAML{}},
		grants: map[string]core.Scope{},
	}
	fresh.SetAuditSink(store.audit)
	for _, b := range backends {
		if err := fresh.SetTier(ctx, b, core.PostureSandbox); err != nil {
			fmt.Fprintf(stderr, "agis policy: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "initialized %s (all backends at sandbox)\n", path)
	return 0
}

// cmdSet applies one rule, to one backend or all of them.
func cmdSet(ctx context.Context, store *Store, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	backend := fs.String("backend", "", "restrict to one backend (default: all)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rest := fs.Args()
	if len(rest) != 3 {
		fmt.Fprintln(stderr, "usage: agis policy set [-b backend] <category> <pattern> <allow|deny>")
		return 1
	}
	category, pattern, action := rest[0], rest[1], rest[2]

	targets := backends
	if *backend != "" {
		targets = []string{*backend}
	}
	for _, b := range targets {
		if err := store.SetRule(ctx, category, b, pattern, action); err != nil {
			fmt.Fprintf(stderr, "agis policy: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "%s %s rule %q for %s\n", action, category, pattern, strings.Join(targets, ","))
	return 0
}

// cmdRm removes exact-matching rules, from one backend or all.
func cmdRm(ctx context.Context, store *Store, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	backend := fs.String("backend", "", "restrict to one backend (default: all)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintln(stderr, "usage: agis policy rm [-b backend] <category> <pattern>")
		return 1
	}
	category, pattern := rest[0], rest[1]

	targets := backends
	if *backend != "" {
		targets = []string{*backend}
	}
	var lastErr error
	for _, b := range targets {
		if err := store.RemoveRule(ctx, category, b, pattern); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		fmt.Fprintf(stderr, "agis policy: %v\n", lastErr)
		return 1
	}
	fmt.Fprintf(stdout, "removed %s rule %q for %s\n", category, pattern, strings.Join(targets, ","))
	return 0
}

// cmdShow prints tiers and the flattened rule table.
func cmdShow(ctx context.Context, store *Store, _ []string, stdout, stderr io.Writer) int {
	if store.Broken() {
		fmt.Fprintf(stderr, "agis policy: %v\n", store.Err())
		return 1
	}

	fmt.Fprintln(stdout, "tiers:")
	for _, b := range backends {
		p := core.Posture(store.file.Tiers[b])
		if p == "" {
			p = core.PostureSandbox
		}
		fmt.Fprintf(stdout, "  %-7s %s\n", b, p)
	}

	rules, err := store.Rules(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "agis policy: %v\n", err)
		return 1
	}
	if len(rules) == 0 {
		fmt.Fprintln(stdout, "rules: (none)")
		return 0
	}
	fmt.Fprintln(stdout, "rules:")
	for _, r := range rules {
		fmt.Fprintf(stdout, "  %-8s %-6s %-24s %s\n", r.Category, r.Backend, r.Pattern, r.Action)
	}
	return 0
}

// cmdTier persists a baseline posture; full is refused with guidance.
func cmdTier(ctx context.Context, store *Store, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: agis policy tier <backend> <sandbox|standard>")
		return 1
	}
	posture := core.Posture(args[1])
	if posture == core.PostureFull {
		fmt.Fprintln(stderr, "agis policy: full is session-only — grant it through the /permisos panel")
		return 1
	}
	if err := store.SetTier(ctx, args[0], posture); err != nil {
		fmt.Fprintf(stderr, "agis policy: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s tier set to %s\n", args[0], posture)
	return 0
}

// cmdTest prints the decision for a subject without executing anything.
func cmdTest(ctx context.Context, store *Store, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	category := fs.String("category", core.CategoryCommands, "commands | files | network")
	backend := fs.String("backend", "local", "local | docker | ssh")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	subject := strings.Join(fs.Args(), " ")
	if subject == "" {
		fmt.Fprintln(stderr, "usage: agis policy test [-c category] [-b backend] <subject...>")
		return 1
	}

	req := core.GuardRequest{Backend: *backend, Category: *category, Subject: subject}
	fmt.Fprintf(stdout, "%s\n", store.Evaluate(ctx, req))
	return 0
}
