package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// RunProfileCLI routes and executes `agis profile` subcommands.
func RunProfileCLI(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	if len(args) == 0 {
		printProfileUsage(stdout)
		return 0
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "help", "-h", "-help", "--help":
		printProfileUsage(stdout)
		return 0
	case "list":
		return runProfileList(subArgs, stdout, stderr)
	case "create":
		return runProfileCreate(subArgs, stdout, stderr)
	case "show":
		return runProfileShow(subArgs, stdout, stderr)
	case "use", "switch":
		return runProfileSwitch(subArgs, stdout, stderr)
	case "delete":
		return runProfileDelete(subArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agis profile: unknown subcommand '%s'\n\n", subcmd)
		printProfileUsage(stderr)
		return 2
	}
}

func printProfileUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis profile <subcommand> [flags] [args]\n\n")
	fmt.Fprintf(w, "Manage isolated AGIS agent profiles (isolated configurations, databases, personas, and skills).\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  list                     List all available profiles\n")
	fmt.Fprintf(w, "  create <name>            Create a new profile (optional: -clone <source>)\n")
	fmt.Fprintf(w, "  show [name]              Display profile paths and metadata\n")
	fmt.Fprintf(w, "  use <name>               Switch active profile (alias: switch)\n")
	fmt.Fprintf(w, "  delete <name>            Delete a profile (optional: -force)\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -json                    Output in JSON format (list, show)\n")
	fmt.Fprintf(w, "  -clone <source>          Source profile to clone from (create)\n")
	fmt.Fprintf(w, "  -force                   Force deletion of active profile (delete)\n")
	fmt.Fprintf(w, "  -h, --help               Show help\n")
}

func runProfileList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonOut := fs.Bool("json", false, "output in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis profile list [-json]\n")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	profiles, err := config.ListProfiles()
	if err != nil {
		fmt.Fprintf(stderr, "agis profile list: %v\n", err)
		return 1
	}

	if *jsonOut {
		data, err := json.MarshalIndent(profiles, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis profile list: marshaling json: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return 0
	}

	fmt.Fprintf(stdout, "%-8s %-20s %s\n", "ACTIVE", "NAME", "PATH")
	fmt.Fprintf(stdout, "%-8s %-20s %s\n", "------", "----", "----")
	for _, p := range profiles {
		activeMarker := ""
		if p.IsActive {
			activeMarker = "* active"
		}
		fmt.Fprintf(stdout, "%-8s %-20s %s\n", activeMarker, p.Name, p.Path)
	}

	return 0
}

func runProfileCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("profile create", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cloneSource := fs.String("clone", "", "Source profile to clone configuration and state from")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis profile create <name> [-clone <source>]\n")
	}

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-clone" || arg == "--clone") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if len(posArgs) < 1 {
		fmt.Fprintf(stderr, "agis profile create: requires a profile name\nUsage: agis profile create <name> [-clone <source>]\n")
		return 2
	}

	name := posArgs[0]
	if err := config.CreateProfile(name, *cloneSource); err != nil {
		fmt.Fprintf(stderr, "agis profile create: %v\n", err)
		return 1
	}

	if *cloneSource != "" {
		fmt.Fprintf(stdout, "Profile '%s' created (cloned from '%s')\n", name, *cloneSource)
	} else {
		fmt.Fprintf(stdout, "Profile '%s' created successfully\n", name)
	}
	return 0
}

func runProfileShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("profile show", flag.ContinueOnError)
	fs.SetOutput(stderr)

	jsonOut := fs.Bool("json", false, "output in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis profile show [name] [-json]\n")
	}

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	targetProfile := ""
	if len(posArgs) > 0 {
		targetProfile = posArgs[0]
	}

	paths := config.ResolveProfilePaths(targetProfile)

	// Validate target profile exists if explicitly requested
	if targetProfile != "" && targetProfile != "default" {
		if _, err := os.Stat(paths.HomeDir); os.IsNotExist(err) {
			fmt.Fprintf(stderr, "agis profile show: profile '%s' does not exist\n", targetProfile)
			return 1
		}
	}

	if *jsonOut {
		data, err := json.MarshalIndent(paths, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis profile show: marshaling json: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return 0
	}

	fmt.Fprintf(stdout, "Profile:     %s\n", paths.ActiveProfileName)
	fmt.Fprintf(stdout, "Base Dir:    %s\n", paths.HomeDir)
	fmt.Fprintf(stdout, "Config:      %s\n", paths.ConfigFile)
	fmt.Fprintf(stdout, "Database:    %s\n", paths.DBFile)
	fmt.Fprintf(stdout, "Soul:        %s\n", paths.SoulFile)
	fmt.Fprintf(stdout, "Skills:      %s\n", paths.SkillsDir)
	fmt.Fprintf(stdout, "Policy:      %s\n", paths.PolicyFile)
	return 0
}

func runProfileSwitch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		if len(args) == 0 {
			fmt.Fprintf(stderr, "agis profile use: requires a profile name\nUsage: agis profile use <name>\n")
			return 2
		}
		fmt.Fprintf(stdout, "Usage: agis profile use <name> (alias: agis profile switch <name>)\n")
		return 0
	}

	name := args[0]
	if err := config.SwitchProfile(name); err != nil {
		fmt.Fprintf(stderr, "agis profile switch: %v\n", err)
		return 1
	}

	if name == "default" || name == "" {
		fmt.Fprintf(stdout, "Switched active profile to 'default' (%s)\n", config.BaseHome())
	} else {
		fmt.Fprintf(stdout, "Switched active profile to '%s' (%s)\n", name, filepath.Join(config.BaseHome(), "profiles", name))
	}
	return 0
}

func runProfileDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("profile delete", flag.ContinueOnError)
	fs.SetOutput(stderr)

	force := fs.Bool("force", false, "Force delete even if currently active")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis profile delete <name> [-force]\n")
	}

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if len(posArgs) < 1 {
		fmt.Fprintf(stderr, "agis profile delete: requires a profile name\nUsage: agis profile delete <name> [-force]\n")
		return 2
	}

	name := posArgs[0]
	if err := config.DeleteProfile(name, *force); err != nil {
		fmt.Fprintf(stderr, "agis profile delete: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Deleted profile '%s'\n", name)
	return 0
}
