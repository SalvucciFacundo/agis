package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/updater"
	"github.com/SalvucciFacundo/agis/internal/version"
)

var (
	testGitHubBaseURL string
	testTargetExePath string
)

// RunUpdateCLI routes the `agis update` self-updater subcommand.
func RunUpdateCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)

	check := fs.Bool("check", false, "check for updates without modifying the binary")
	backup := fs.Bool("backup", false, "backup $AGIS_HOME state before updating")
	targetVer := fs.String("version", "", "target specific release version tag (e.g. v0.4.0)")
	force := fs.Bool("force", false, "force update even if binary is up to date")
	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")

	fs.Usage = func() {
		printUpdateUsage(stdout)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis update: %v\n", err)
		return 2
	}

	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "agis update: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		printUpdateUsage(stderr)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var clientOpts []updater.Option
	if testGitHubBaseURL != "" {
		clientOpts = append(clientOpts, updater.WithBaseURL(testGitHubBaseURL))
	}
	client := updater.NewClient("SalvucciFacundo/agis", clientOpts...)

	var rel *updater.Release
	var err error
	if *targetVer != "" {
		rel, err = client.FetchReleaseByTag(ctx, *targetVer)
	} else {
		rel, err = client.FetchLatestRelease(ctx)
	}
	if err != nil {
		fmt.Fprintf(stderr, "agis update: fetching release metadata: %v\n", err)
		return 1
	}

	currVer := version.Version
	targetVersion := rel.TagName
	isNewer := version.IsNewer(targetVersion, currVer)

	if *check {
		if isNewer {
			fmt.Fprintf(stdout, "Update available: %s -> %s\n", currVer, targetVersion)
		} else {
			fmt.Fprintf(stdout, "agis is already up to date (%s)\n", currVer)
		}
		return 0
	}

	if !isNewer && !*force {
		fmt.Fprintf(stdout, "agis is already up to date (%s). Use --force to re-install.\n", currVer)
		return 0
	}

	if *backup {
		agisHome := config.AgisHome()
		if *configPath != "" {
			if cfg, err := config.Load(*configPath); err == nil && cfg.DB.Path != "" {
				// config loaded
			}
		}
		archivePath, err := updater.CreateBackup(agisHome, "")
		if err != nil {
			fmt.Fprintf(stderr, "agis update: backup failed: %v\n", err)
			return 1
		}
		fmt.Fprintf(stderr, "Created backup archive at %s\n", archivePath)
	}

	binAsset, chkAsset, err := updater.FindAssetForPlatform(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fmt.Fprintf(stderr, "agis update: resolving platform assets: %v\n", err)
		return 1
	}

	binBytes, err := client.DownloadAsset(ctx, binAsset)
	if err != nil {
		fmt.Fprintf(stderr, "agis update: downloading binary asset: %v\n", err)
		return 1
	}

	chkBytes, err := client.DownloadAsset(ctx, chkAsset)
	if err != nil {
		fmt.Fprintf(stderr, "agis update: downloading checksums: %v\n", err)
		return 1
	}

	if err := updater.VerifyChecksum(binBytes, binAsset.Name, string(chkBytes)); err != nil {
		fmt.Fprintf(stderr, "agis update: checksum verification failed: %v\n", err)
		return 1
	}

	rawBin, err := updater.ExtractBinaryFromAsset(binAsset.Name, binBytes)
	if err != nil {
		fmt.Fprintf(stderr, "agis update: extracting binary from asset: %v\n", err)
		return 1
	}

	exePath := testTargetExePath
	if exePath == "" {
		exePath, err = os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "agis update: determining current executable path: %v\n", err)
			return 1
		}
	}

	if err := updater.ApplyBinary(rawBin, exePath); err != nil {
		fmt.Fprintf(stderr, "agis update: applying binary: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Successfully updated agis to %s\n", targetVersion)
	return 0
}

func printUpdateUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis update [flags]\n\n")
	fmt.Fprintf(w, "Self-update the AGIS binary from GitHub releases with checksum verification and backup.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -check           Check for available updates without modifying the executable\n")
	fmt.Fprintf(w, "  -backup          Create a .tar.gz snapshot of $AGIS_HOME state before updating\n")
	fmt.Fprintf(w, "  -version string  Target specific release version tag (e.g. v0.4.0)\n")
	fmt.Fprintf(w, "  -force           Force re-download and replacement even if already up to date\n")
	fmt.Fprintf(w, "  -config string   Path to custom config file\n")
	fmt.Fprintf(w, "  -h, --help       Show help\n")
}
