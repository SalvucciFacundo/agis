package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/doctor"
)

// RunDoctorCLI routes the `agis doctor` diagnostic subcommand.
func RunDoctorCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")
	jsonOutput := fs.Bool("json", false, "output health report in JSON format")
	noColor := fs.Bool("no-color", false, "disable ANSI color output")

	fs.Usage = func() {
		printDoctorUsage(stdout)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis doctor: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis doctor: loading configuration: %v\n", err)
		return 2
	}

	doc := doctor.New(cfg)
	report := doc.Run(context.Background())

	if *jsonOutput {
		data, err := report.JSON()
		if err != nil {
			fmt.Fprintf(stderr, "agis doctor: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
	} else {
		useColor := !*noColor && isTerminal(stdout)
		fmt.Fprint(stdout, report.Format(useColor))
	}

	if report.HasFailures() {
		return 1
	}
	return 0
}

func printDoctorUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis doctor [flags]\n\n")
	fmt.Fprintf(w, "Diagnose environment, configuration, SQLite storage, LLM connectivity, and subsystem health.\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -config string   Path to config file\n")
	fmt.Fprintf(w, "  -json            Output report in JSON format\n")
	fmt.Fprintf(w, "  -no-color        Disable ANSI color output\n")
	fmt.Fprintf(w, "  -h, --help       Show help\n")
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			return true
		}
	}
	return false
}
