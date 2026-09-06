package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/skills"
)

// RunSkillCLI routes the `agis skill` subcommand router.
func RunSkillCLI(args []string, stdout, stderr io.Writer) int {
	return RunSkillCLIWithIn(args, os.Stdin, stdout, stderr)
}

// RunSkillCLIWithIn routes the `agis skill` subcommand router with an explicit stdin reader.
func RunSkillCLIWithIn(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSkillUsage(stdout)
		return 0
	}

	subcommand := args[0]
	if subcommand == "-h" || subcommand == "--help" || subcommand == "-help" || subcommand == "help" {
		printSkillUsage(stdout)
		return 0
	}

	subArgs := args[1:]

	switch subcommand {
	case "list", "ls":
		return runSkillList(subArgs, stdout, stderr)
	case "create", "new", "add":
		return runSkillCreate(subArgs, stdout, stderr)
	case "show", "get", "cat", "view":
		return runSkillShow(subArgs, stdout, stderr)
	case "delete", "remove", "rm":
		return runSkillDelete(subArgs, stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agis skill: unknown subcommand '%s'\n", subcommand)
		printSkillUsage(stderr)
		return 2
	}
}

func initSkillHub(configPath string, stderr io.Writer) (*config.Config, core.Repository, *skills.Hub, string, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis skill: loading configuration: %v\n", err)
		return nil, nil, nil, "", nil, err
	}

	ctx := context.Background()
	repo, err := memory.NewRepository(ctx, cfg.DB.Path)
	if err != nil {
		fmt.Fprintf(stderr, "agis skill: opening repository: %v\n", err)
		return nil, nil, nil, "", nil, err
	}

	resolvedDir := cfg.Skills.Dir
	if resolvedDir == "" {
		resolvedDir = filepath.Join(config.AgisHome(), "skills")
	}

	hub := skills.NewHub(repo, slog.Default())
	_ = hub.LoadDir(ctx, resolvedDir)

	regDir := filepath.Join(config.AgisHome(), ".atl")
	if mkErr := os.MkdirAll(regDir, 0o700); mkErr == nil {
		hub.SyncRegistry(filepath.Join(regDir, "skill-registry.md"))
	}

	cleanup := func() {
		_ = repo.Close()
	}
	return cfg, repo, hub, resolvedDir, cleanup, nil
}

func runSkillList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill list", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	jsonOutput := fs.Bool("json", false, "output skills in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis skill list [flags]\n\n")
		fmt.Fprintf(stdout, "List all installed skills.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis skill list: %v\n", err)
		return 2
	}

	_, _, hub, _, cleanup, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	list := hub.Skills()
	if list == nil {
		list = []core.Skill{}
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis skill list: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if len(list) == 0 {
		fmt.Fprintln(stdout, "No skills found. Use 'agis skill create <name>' to create one.")
		return 0
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tTRIGGER\tSOURCE\tUSES\tDESCRIPTION")
	for _, s := range list {
		desc := s.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		trigger := s.Trigger
		if trigger == "" {
			trigger = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", s.Name, trigger, s.Source, s.UsageCount, desc)
	}
	_ = w.Flush()
	return 0
}

func runSkillCreate(args []string, stdout, stderr io.Writer) int {
	var positionalArgs []string
	var flagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-desc" || arg == "--desc" || arg == "-trigger" || arg == "--trigger" || arg == "-config" || arg == "--config") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs := flag.NewFlagSet("skill create", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	desc := fs.String("desc", "", "short description of the skill")
	trigger := fs.String("trigger", "", "trigger keywords for the skill")
	force := fs.Bool("force", false, "overwrite existing skill if present")
	overwrite := fs.Bool("overwrite", false, "alias for -force")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis skill create <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Scaffold a new agentskills.io-compliant skill.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis skill create: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "agis skill create: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	if err := skills.ValidateSkillName(name); err != nil {
		fmt.Fprintf(stderr, "agis skill create: %v\n", err)
		return 2
	}

	_, _, hub, resolvedDir, cleanup, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	canOverwrite := *force || *overwrite

	// Check if already exists
	if !canOverwrite {
		if _, exists := hub.GetSkill(name); exists {
			fmt.Fprintf(stderr, "agis skill create: skill %q already exists (use -force to overwrite)\n", name)
			return 1
		}
	}

	skillDir := filepath.Join(resolvedDir, name)
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		fmt.Fprintf(stderr, "agis skill create: creating directory: %v\n", err)
		return 1
	}

	description := *desc
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Procedural instructions for %s", name)
	}

	content := fmt.Sprintf(`---
name: %s
description: %s
trigger: %s
license: Apache-2.0
metadata:
  author: user
  version: "1.0"
---

## When to Use
Describe when this skill should be applied by the agent.

## Critical Rules
1. Follow standard operational guidelines.

## Workflow
1. Step 1: Initial preparation.
2. Step 2: Main execution procedure.

## Examples
Provide concrete examples of input scenarios and expected outcomes.
`, name, description, *trigger)

	filePath := filepath.Join(skillDir, "SKILL.md")
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o600); err != nil {
		fmt.Fprintf(stderr, "agis skill create: writing file: %v\n", err)
		return 1
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(stderr, "agis skill create: finalizing file: %v\n", err)
		return 1
	}

	ctx := context.Background()
	_ = hub.Reload(ctx, resolvedDir)
	regDir := filepath.Join(config.AgisHome(), ".atl")
	hub.SyncRegistry(filepath.Join(regDir, "skill-registry.md"))

	fmt.Fprintf(stdout, "Skill %q created successfully at %s\n", name, filePath)
	return 0
}

func runSkillShow(args []string, stdout, stderr io.Writer) int {
	var positionalArgs []string
	var flagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-config" || arg == "--config") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs := flag.NewFlagSet("skill show", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	raw := fs.Bool("raw", false, "display raw skill file with frontmatter")
	jsonOutput := fs.Bool("json", false, "output skill details in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis skill show <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Show detailed information and instructions for a skill.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis skill show: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "agis skill show: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	_, _, hub, resolvedDir, cleanup, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	skill, ok := hub.GetSkill(name)
	if !ok {
		fmt.Fprintf(stderr, "agis skill show: skill %q not found\n", name)
		return 1
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(skill, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis skill show: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if *raw {
		// Look for file on disk
		paths := []string{
			filepath.Join(resolvedDir, name, "SKILL.md"),
			filepath.Join(resolvedDir, name, name+".md"),
			filepath.Join(resolvedDir, name+".md"),
		}
		var rawBytes []byte
		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil {
				rawBytes = data
				break
			}
		}
		if len(rawBytes) > 0 {
			fmt.Fprint(stdout, string(rawBytes))
		} else {
			fmt.Fprintf(stdout, "---\nname: %s\ndescription: %s\ntrigger: %s\n---\n\n%s\n",
				skill.Name, skill.Description, skill.Trigger, skill.Content)
		}
		return 0
	}

	fmt.Fprintf(stdout, "Skill:       %s\n", skill.Name)
	fmt.Fprintf(stdout, "Description: %s\n", skill.Description)
	if skill.Trigger != "" {
		fmt.Fprintf(stdout, "Trigger:     %s\n", skill.Trigger)
	}
	fmt.Fprintf(stdout, "Source:      %s\n", skill.Source)
	fmt.Fprintf(stdout, "Usage Count: %d\n", skill.UsageCount)
	fmt.Fprintf(stdout, "\n%s\n", skill.Content)
	return 0
}

func runSkillDelete(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var positionalArgs []string
	var flagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-config" || arg == "--config") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flagArgs = append(flagArgs, args[i+1])
				i++
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs := flag.NewFlagSet("skill delete", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	yes := fs.Bool("yes", false, "confirm deletion without interactive prompt")
	force := fs.Bool("force", false, "alias for -yes")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis skill delete <name> [flags]\n\n")
		fmt.Fprintf(stdout, "Delete an installed skill from disk and repository.\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis skill delete: %v\n", err)
		return 2
	}

	if len(positionalArgs) == 0 {
		fmt.Fprintf(stderr, "agis skill delete: skill name is required\n")
		fs.Usage()
		return 2
	}

	name := positionalArgs[0]
	_, repo, hub, resolvedDir, cleanup, err := initSkillHub(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	if _, ok := hub.GetSkill(name); !ok {
		fmt.Fprintf(stderr, "agis skill delete: skill %q not found\n", name)
		return 1
	}

	confirmed := *yes || *force
	if !confirmed {
		fmt.Fprintf(stdout, "Are you sure you want to delete skill %q? [y/N]: ", name)
		reader := bufio.NewReader(stdin)
		input, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(stderr, "agis skill delete: reading confirmation: %v\n", err)
			return 1
		}
		trimmed := strings.ToLower(strings.TrimSpace(input))
		if trimmed != "y" && trimmed != "yes" {
			fmt.Fprintln(stdout, "Deletion canceled.")
			return 0
		}
	}

	// Delete from filesystem
	nestedDir := filepath.Join(resolvedDir, name)
	flatFile := filepath.Join(resolvedDir, name+".md")
	_ = os.RemoveAll(nestedDir)
	_ = os.Remove(flatFile)

	// Delete from database
	ctx := context.Background()
	_ = repo.DeleteSkill(ctx, name)

	// Reload hub and sync registry
	_ = hub.Reload(ctx, resolvedDir)
	regDir := filepath.Join(config.AgisHome(), ".atl")
	hub.SyncRegistry(filepath.Join(regDir, "skill-registry.md"))

	fmt.Fprintf(stdout, "Skill %q deleted successfully.\n", name)
	return 0
}

func printSkillUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis skill <command> [flags]\n\n")
	fmt.Fprintf(w, "Manage AGIS skills, discovery, and agentskills.io conformance.\n\n")
	fmt.Fprintf(w, "Commands:\n")
	fmt.Fprintf(w, "  list     List all installed skills\n")
	fmt.Fprintf(w, "  create   Scaffold a new skill\n")
	fmt.Fprintf(w, "  show     Show skill details and instructions\n")
	fmt.Fprintf(w, "  delete   Delete an installed skill\n\n")
	fmt.Fprintf(w, "Use 'agis skill <command> --help' for more information about a command.\n")
}
