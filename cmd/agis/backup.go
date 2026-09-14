package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/backup"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/version"
)

// RunBackupCLI executes the `agis backup` subcommand.
func RunBackupCLI(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(stdout)

	outputFlag := fs.String("o", "", "output archive file path (default: agis-<profile>-<timestamp>.tar.gz)")
	configPath := fs.String("config", "", "path to config file")

	fs.Usage = func() {
		printBackupUsage(stdout)
	}

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				name := strings.TrimLeft(arg, "-")
				if name != "h" && name != "help" {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis backup: %v\n", err)
		return 2
	}

	if *configPath != "" {
		_, _ = config.Load(*configPath)
	}

	var profileName string
	if len(posArgs) > 0 {
		profileName = strings.TrimSpace(posArgs[0])
	}
	if profileName == "" {
		profileName = config.ActiveProfile()
		if profileName == "" {
			profileName = "default"
		}
	}

	outPath := *outputFlag
	if outPath == "" {
		timestamp := time.Now().Format("20060102-150405")
		outPath = fmt.Sprintf("agis-%s-%s.tar.gz", profileName, timestamp)
	}

	// Ensure destination directory exists if path contains directories
	if dir := filepath.Dir(outPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			fmt.Fprintf(stderr, "agis backup: creating output directory: %v\n", err)
			return 1
		}
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis backup: creating output file %q: %v\n", outPath, err)
		return 1
	}
	defer outFile.Close()

	start := time.Now()
	manifest, err := backup.Create(profileName, outFile, backup.CreateOptions{
		BaseHome:    config.BaseHome(),
		AGISVersion: version.Version,
	})
	if err != nil {
		_ = os.Remove(outPath)
		fmt.Fprintf(stderr, "agis backup error: %v\n", err)
		return 1
	}

	duration := time.Since(start)
	var totalBytes int64
	for _, f := range manifest.Files {
		totalBytes += f.Size
	}

	fmt.Fprintf(stdout, "Backup complete: %s\n", outPath)
	fmt.Fprintf(stdout, "Profile:         %s\n", manifest.SourceProfile)
	fmt.Fprintf(stdout, "Files archived:  %d\n", len(manifest.Files))
	fmt.Fprintf(stdout, "Uncompressed:    %.2f KB\n", float64(totalBytes)/1024.0)
	fmt.Fprintf(stdout, "Time elapsed:    %v\n", duration.Round(time.Millisecond))
	return 0
}

func printBackupUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis backup [profile] [flags]\n\n")
	fmt.Fprintf(w, "Create a portable .tar.gz archive of an AGIS agent profile.\n\n")
	fmt.Fprintf(w, "Arguments:\n")
	fmt.Fprintf(w, "  [profile]        Name of profile to backup (default: active profile)\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -o string        Path for the output .tar.gz archive (default: agis-<profile>-<timestamp>.tar.gz)\n")
	fmt.Fprintf(w, "  -config string   Path to config file\n")
}

// RunRestoreCLI executes the `agis restore` subcommand.
func RunRestoreCLI(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(stdout)

	targetProfile := fs.String("profile", "", "target profile name (default: source profile from archive manifest)")
	force := fs.Bool("force", false, "overwrite existing profile if present")
	fs.BoolVar(force, "f", false, "alias for -force")
	configPath := fs.String("config", "", "path to config file")

	fs.Usage = func() {
		printRestoreUsage(stdout)
	}

	var flagArgs, posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				name := strings.TrimLeft(arg, "-")
				if name != "f" && name != "force" && name != "h" && name != "help" {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis restore: %v\n", err)
		return 2
	}

	if *configPath != "" {
		_, _ = config.Load(*configPath)
	}

	if len(posArgs) == 0 {
		fmt.Fprintf(stderr, "agis restore: missing archive path argument\n\n")
		printRestoreUsage(stderr)
		return 2
	}

	archivePath := strings.TrimSpace(posArgs[0])

	archiveFile, err := os.Open(archivePath)
	if err != nil {
		fmt.Fprintf(stderr, "agis restore: opening archive %q: %v\n", archivePath, err)
		return 1
	}
	defer archiveFile.Close()

	start := time.Now()
	report, err := backup.Restore(archiveFile, *targetProfile, backup.RestoreOptions{
		BaseHome: config.BaseHome(),
		Force:    *force,
	})
	if err != nil {
		fmt.Fprintf(stderr, "agis restore error: %v\n", err)
		return 1
	}

	duration := time.Since(start)
	fmt.Fprintf(stdout, "Profile restored successfully!\n")
	fmt.Fprintf(stdout, "Profile Name:    %s\n", report.ProfileName)
	fmt.Fprintf(stdout, "Target Dir:      %s\n", report.TargetDir)
	fmt.Fprintf(stdout, "Files Restored:  %d\n", report.FilesCount)
	fmt.Fprintf(stdout, "Time Elapsed:    %v\n", duration.Round(time.Millisecond))
	return 0
}

func printRestoreUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis restore <archive.tar.gz> [flags]\n\n")
	fmt.Fprintf(w, "Restore an AGIS agent profile from a .tar.gz archive.\n\n")
	fmt.Fprintf(w, "Arguments:\n")
	fmt.Fprintf(w, "  <archive.tar.gz>  Path to the profile backup archive\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -profile string   Target profile name (default: original profile name from archive)\n")
	fmt.Fprintf(w, "  -force, -f        Overwrite target profile if it already exists\n")
	fmt.Fprintf(w, "  -config string    Path to config file\n")
}
