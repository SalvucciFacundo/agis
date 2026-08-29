package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/plugins"
)

// RunPluginsCLI runs the `agis plugins` subcommand router.
func RunPluginsCLI(args []string, stdout, stderr io.Writer) int {
	subcommand := "list"
	var targetPlugin string
	var flagArgs []string

	if len(args) > 0 {
		switch args[0] {
		case "list":
			subcommand = "list"
			flagArgs = args[1:]
		case "enable", "disable", "inspect":
			subcommand = args[0]
			if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
				targetPlugin = args[1]
				flagArgs = args[2:]
			} else {
				flagArgs = args[1:]
			}
		case "-h", "--help", "-help":
			printPluginsUsage(stdout)
			return 0
		default:
			if strings.HasPrefix(args[0], "-") {
				flagArgs = args
			} else {
				fmt.Fprintf(stderr, "agis plugins: unknown subcommand %q\n", args[0])
				printPluginsUsage(stderr)
				return 2
			}
		}
	}

	fs := flag.NewFlagSet("plugins "+subcommand, flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")
	pluginsDir := fs.String("dir", "", "path to plugins directory (overrides config)")

	fs.Usage = func() {
		printPluginsUsage(stdout)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis plugins: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis plugins: loading config: %v\n", err)
		return 1
	}

	resolvedDir := *pluginsDir
	if resolvedDir == "" {
		resolvedDir = cfg.Plugins.Dir
	}
	if resolvedDir == "" {
		resolvedDir = filepath.Join(config.AgisHome(), "plugins")
	}

	logger := slog.New(slog.NewTextHandler(stderr, nil))
	mgr := plugins.NewManager(
		plugins.WithStateDir(resolvedDir),
		plugins.WithLogger(logger),
	)

	if err := mgr.Load(resolvedDir); err != nil {
		fmt.Fprintf(stderr, "agis plugins: loading plugins: %v\n", err)
		return 1
	}

	switch subcommand {
	case "list":
		return runPluginsList(mgr, resolvedDir, stdout)
	case "enable":
		if targetPlugin == "" {
			fmt.Fprintf(stderr, "agis plugins enable: plugin name is required\n")
			return 2
		}
		if err := mgr.Enable(targetPlugin); err != nil {
			fmt.Fprintf(stderr, "agis plugins enable: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Plugin %q enabled.\n", targetPlugin)
		return 0
	case "disable":
		if targetPlugin == "" {
			fmt.Fprintf(stderr, "agis plugins disable: plugin name is required\n")
			return 2
		}
		if err := mgr.Disable(targetPlugin); err != nil {
			fmt.Fprintf(stderr, "agis plugins disable: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Plugin %q disabled.\n", targetPlugin)
		return 0
	case "inspect":
		if targetPlugin == "" {
			fmt.Fprintf(stderr, "agis plugins inspect: plugin name is required\n")
			return 2
		}
		return runPluginsInspect(mgr, targetPlugin, stdout, stderr)
	default:
		printPluginsUsage(stdout)
		return 2
	}
}

func printPluginsUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis plugins [list|enable|disable|inspect] [args] [flags]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  list            List all discovered plugins and their statuses (default)\n")
	fmt.Fprintf(w, "  enable <name>   Enable a plugin by name\n")
	fmt.Fprintf(w, "  disable <name>  Disable a plugin by name\n")
	fmt.Fprintf(w, "  inspect <name>  Show detailed manifest metadata for a plugin\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -config string\n")
	fmt.Fprintf(w, "        path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)\n")
	fmt.Fprintf(w, "  -dir string\n")
	fmt.Fprintf(w, "        path to plugins directory\n")
}

func runPluginsList(mgr *plugins.Manager, dir string, stdout io.Writer) int {
	list := mgr.List()
	if len(list) == 0 {
		fmt.Fprintf(stdout, "No plugins found in %s\n", dir)
		return 0
	}

	fmt.Fprintf(stdout, "Discovered Plugins (%d):\n", len(list))
	for _, p := range list {
		status := "disabled"
		if p.Enabled {
			status = "enabled"
		}
		desc := p.Manifest.Description
		if desc != "" {
			desc = " - " + desc
		}
		fmt.Fprintf(stdout, "  • %-20s (v%-8s) [%s]%s\n", p.Manifest.Name, p.Manifest.Version, status, desc)
	}
	return 0
}

func runPluginsInspect(mgr *plugins.Manager, name string, stdout, stderr io.Writer) int {
	p, err := mgr.Get(name)
	if err != nil {
		fmt.Fprintf(stderr, "agis plugins inspect: %v\n", err)
		return 1
	}

	status := "disabled"
	if p.Enabled {
		status = "enabled"
	}

	fmt.Fprintf(stdout, "Plugin: %s\n", p.Manifest.Name)
	fmt.Fprintf(stdout, "  Version:     %s\n", p.Manifest.Version)
	fmt.Fprintf(stdout, "  Status:      %s\n", status)
	fmt.Fprintf(stdout, "  Directory:   %s\n", p.Dir)
	if p.Manifest.Description != "" {
		fmt.Fprintf(stdout, "  Description: %s\n", p.Manifest.Description)
	}
	if p.Manifest.Entrypoint != "" {
		fmt.Fprintf(stdout, "  Entrypoint:  %s\n", p.Manifest.Entrypoint)
	}
	if len(p.Manifest.Tools) > 0 {
		fmt.Fprintf(stdout, "  Tools (%d):\n", len(p.Manifest.Tools))
		for _, t := range p.Manifest.Tools {
			fmt.Fprintf(stdout, "    - %s: %s\n", t.Name, t.Description)
		}
	}
	if len(p.Manifest.Skills) > 0 {
		fmt.Fprintf(stdout, "  Skills (%d): %s\n", len(p.Manifest.Skills), strings.Join(p.Manifest.Skills, ", "))
	}
	if len(p.Manifest.Permissions) > 0 {
		fmt.Fprintf(stdout, "  Permissions: %s\n", strings.Join(p.Manifest.Permissions, ", "))
	}
	return 0
}
