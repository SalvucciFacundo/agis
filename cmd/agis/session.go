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
	"strings"
	"text/tabwriter"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/scan"
	"github.com/SalvucciFacundo/agis/internal/session"
)

// RunSessionCLI routes the `agis session` subcommand router.
func RunSessionCLI(args []string, stdout, stderr io.Writer) int {
	return RunSessionCLIWithIn(args, os.Stdin, stdout, stderr)
}

// RunSessionCLIWithIn routes the `agis session` subcommand router with an explicit stdin reader.
func RunSessionCLIWithIn(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printSessionUsage(stdout)
		return 0
	}

	subcommand := args[0]
	if subcommand == "-h" || subcommand == "--help" || subcommand == "-help" || subcommand == "help" {
		printSessionUsage(stdout)
		return 0
	}

	subArgs := args[1:]

	switch subcommand {
	case "list":
		return runSessionList(subArgs, stdout, stderr)
	case "show":
		return runSessionShow(subArgs, stdout, stderr)
	case "delete":
		return runSessionDelete(subArgs, stdin, stdout, stderr)
	case "rename":
		return runSessionRename(subArgs, stdout, stderr)
	case "export":
		return runSessionExport(subArgs, stdout, stderr)
	case "snapshot":
		return runSessionSnapshot(subArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agis session: unknown subcommand '%s'\n", subcommand)
		printSessionUsage(stderr)
		return 2
	}
}

func initSessionManager(configPath string, stderr io.Writer) (*session.Manager, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis session: loading configuration: %v\n", err)
		return nil, nil, err
	}

	ctx := context.Background()
	var repoOpts []memory.Option
	if cfg.Embeddings.Enabled {
		embedder, err := llm.NewEmbedder(cfg.Embeddings, cfg.LLM.APIKey)
		if err != nil {
			slog.Warn("embeddings: initializing embedder (falling back to FTS5)", "error", err)
		} else {
			repoOpts = append(repoOpts, memory.WithEmbedder(embedder))
		}
	}

	repo, err := memory.NewRepository(ctx, cfg.DB.Path, repoOpts...)
	if err != nil {
		fmt.Fprintf(stderr, "agis session: opening repository: %v\n", err)
		return nil, nil, err
	}

	mgr := session.New(repo, nil, slog.Default())
	cleanup := func() {
		_ = repo.Close()
	}
	return mgr, cleanup, nil
}

func runSessionList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session list", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	jsonOutput := fs.Bool("json", false, "output sessions in JSON format")
	limit := fs.Int("limit", 20, "maximum number of sessions to list")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session list [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -limit int       Maximum number of sessions to return (default 20)\n")
		fmt.Fprintf(stdout, "  -json            Output sessions as JSON\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session list: %v\n", err)
		return 2
	}

	if *limit <= 0 {
		fmt.Fprintf(stderr, "agis session list: -limit must be greater than 0, got %d\n", *limit)
		return 2
	}

	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	convs, err := mgr.List(context.Background(), *limit)
	if err != nil {
		fmt.Fprintf(stderr, "agis session list: %v\n", err)
		return 1
	}

	if *jsonOutput {
		if convs == nil {
			convs = []core.Conversation{}
		}
		data, err := json.MarshalIndent(convs, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis session list: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	if len(convs) == 0 {
		fmt.Fprintln(stdout, "No sessions found.")
		return 0
	}

	w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tMESSAGES\tUPDATED")
	for _, c := range convs {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			c.ID,
			c.Title,
			c.MessageCount,
			c.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	_ = w.Flush()
	return 0
}

func runSessionShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session show", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	jsonOutput := fs.Bool("json", false, "output session details as JSON")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session show <id> [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -json            Output full session as JSON\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if (args[i] == "-config" || args[i] == "--config") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session show: %v\n", err)
		return 2
	}

	if len(positional) != 1 {
		fmt.Fprintf(stderr, "agis session show: requires exactly one session ID argument\n")
		fs.Usage()
		return 2
	}

	sessionID := positional[0]
	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	conv, msgs, err := mgr.Show(context.Background(), sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "agis session show: conversation '%s' not found: %v\n", sessionID, err)
		return 1
	}

	if *jsonOutput {
		payload := map[string]any{
			"conversation": conv,
			"messages":     msgs,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis session show: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "Session: %s (%s)\n", conv.Title, conv.ID)
	fmt.Fprintf(stdout, "Created: %s | Updated: %s | Messages: %d\n",
		conv.CreatedAt.Format("2006-01-02 15:04:05"),
		conv.UpdatedAt.Format("2006-01-02 15:04:05"),
		conv.MessageCount,
	)
	if conv.Summary != "" {
		fmt.Fprintf(stdout, "Summary: %s\n", conv.Summary)
	}
	fmt.Fprintln(stdout, strings.Repeat("-", 60))

	for _, m := range msgs {
		fmt.Fprintf(stdout, "\n[%s] [%s]\n%s\n",
			m.CreatedAt.Format("2006-01-02 15:04:05"),
			strings.ToUpper(string(m.Role)),
			m.Content,
		)
		if len(m.Attachments) > 0 {
			fmt.Fprintln(stdout, "Attachments:")
			for _, att := range m.Attachments {
				if att.URL != "" {
					fmt.Fprintf(stdout, "  - %s (%s): %s\n", att.Name, att.MimeType, att.URL)
				} else {
					fmt.Fprintf(stdout, "  - %s (%s)\n", att.Name, att.MimeType)
				}
			}
		}
	}
	return 0
}

func isTerminalReader(r io.Reader) bool {
	if r == nil {
		return false
	}
	if inter, ok := r.(interface{ IsInteractiveTerminal() bool }); ok {
		return inter.IsInteractiveTerminal()
	}
	if f, ok := r.(*os.File); ok {
		stat, err := f.Stat()
		if err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
			return true
		}
	}
	return false
}

func runSessionDelete(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session delete", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	yesFlag := fs.Bool("yes", false, "skip interactive confirmation prompt")
	yFlag := fs.Bool("y", false, "skip interactive confirmation prompt (shorthand)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session delete <id> [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -yes, -y         Skip interactive confirmation\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if (args[i] == "-config" || args[i] == "--config") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session delete: %v\n", err)
		return 2
	}

	if len(positional) != 1 {
		fmt.Fprintf(stderr, "agis session delete: requires exactly one session ID argument\n")
		fs.Usage()
		return 2
	}

	sessionID := positional[0]
	skipConfirm := *yesFlag || *yFlag

	if !skipConfirm {
		if !isTerminalReader(stdin) {
			fmt.Fprintf(stderr, "confirmation required: use --yes in non-interactive mode\n")
			return 1
		}
		fmt.Fprintf(stderr, "Delete session '%s'? [y/N]: ", sessionID)
		reader := bufio.NewReader(stdin)
		ans, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Fprintf(stderr, "agis session delete: reading input: %v\n", err)
			return 1
		}
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			return 0
		}
	}

	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	if err := mgr.Delete(context.Background(), sessionID); err != nil {
		fmt.Fprintf(stderr, "agis session delete: deleting session '%s': %v\n", sessionID, err)
		return 1
	}

	fmt.Fprintf(stdout, "Deleted session '%s'\n", sessionID)
	return 0
}

func runSessionRename(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session rename", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session rename <id> <title> [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if (args[i] == "-config" || args[i] == "--config") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session rename: %v\n", err)
		return 2
	}

	if len(positional) < 2 {
		fmt.Fprintf(stderr, "agis session rename: requires <id> and <title> arguments\n")
		fs.Usage()
		return 2
	}

	sessionID := positional[0]
	newTitle := strings.Join(positional[1:], " ")

	clean, dropped := scan.Lines(newTitle)
	if dropped > 0 {
		fmt.Fprintf(stderr, "agis session rename: warning: dropped %d injected lines from title\n", dropped)
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		fmt.Fprintf(stderr, "agis session rename: title must not be empty\n")
		return 1
	}

	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	if err := mgr.Rename(context.Background(), sessionID, newTitle); err != nil {
		fmt.Fprintf(stderr, "agis session rename: renaming session '%s': %v\n", sessionID, err)
		return 1
	}

	fmt.Fprintf(stdout, "Renamed session '%s' to '%s'\n", sessionID, clean)
	return 0
}

func runSessionExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session export", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	format := fs.String("format", "markdown", "export format (json, markdown, txt)")
	outputPath := fs.String("output", "", "destination file path (writes to stdout if empty)")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session export <id> [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -format string   Export format: json, markdown, txt (default: markdown)\n")
		fmt.Fprintf(stdout, "  -output string   Destination file path\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if (args[i] == "-config" || args[i] == "--config" ||
				args[i] == "-format" || args[i] == "--format" ||
				args[i] == "-output" || args[i] == "--output") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session export: %v\n", err)
		return 2
	}

	if len(positional) != 1 {
		fmt.Fprintf(stderr, "agis session export: requires exactly one session ID argument\n")
		fs.Usage()
		return 2
	}

	sessionID := positional[0]
	normFormat := strings.ToLower(strings.TrimSpace(*format))
	switch normFormat {
	case "json", "markdown", "md", "txt", "plaintext":
		// valid
	default:
		fmt.Fprintf(stderr, "invalid export format '%s': supported formats are json, markdown, txt\n", *format)
		return 2
	}

	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	data, err := mgr.Export(context.Background(), sessionID, session.ExportFormat(normFormat))
	if err != nil {
		fmt.Fprintf(stderr, "agis session export: exporting session '%s': %v\n", sessionID, err)
		return 1
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, data, 0644); err != nil {
			fmt.Fprintf(stderr, "agis session export: writing output file '%s': %v\n", *outputPath, err)
			return 1
		}
		return 0
	}

	_, _ = stdout.Write(data)
	return 0
}

func runSessionSnapshot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("session snapshot", flag.ContinueOnError)
	fs.SetOutput(stdout)

	configPath := fs.String("config", "", "path to config file")
	jsonOutput := fs.Bool("json", false, "output created snapshot metadata as JSON")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis session snapshot <id> [flags]\n\n")
		fmt.Fprintf(stdout, "Flags:\n")
		fmt.Fprintf(stdout, "  -json            Output created snapshot as JSON\n")
		fmt.Fprintf(stdout, "  -config string   Path to config file\n")
	}

	var positional []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flagArgs = append(flagArgs, args[i])
			if (args[i] == "-config" || args[i] == "--config") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, args[i])
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis session snapshot: %v\n", err)
		return 2
	}

	if len(positional) != 1 {
		fmt.Fprintf(stderr, "agis session snapshot: requires exactly one session ID argument\n")
		fs.Usage()
		return 2
	}

	sessionID := positional[0]
	mgr, cleanup, err := initSessionManager(*configPath, stderr)
	if err != nil {
		return 1
	}
	defer cleanup()

	snap, err := mgr.SnapshotSession(context.Background(), sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "agis session snapshot: snapshotting session '%s': %v\n", sessionID, err)
		return 1
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis session snapshot: serializing JSON: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "Snapshot '%s' created for session '%s'\n", snap.ID, sessionID)
	return 0
}

func printSessionUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis session <subcommand> [flags]\n\n")
	fmt.Fprintf(w, "Manage AGIS conversation sessions, export histories, and trigger backups.\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  list             List conversation sessions\n")
	fmt.Fprintf(w, "  show <id>        Display details and message history of a session\n")
	fmt.Fprintf(w, "  delete <id>      Permanently delete a conversation session\n")
	fmt.Fprintf(w, "  rename <id> <title> Rename a conversation session\n")
	fmt.Fprintf(w, "  export <id>      Export session history (json, markdown, txt)\n")
	fmt.Fprintf(w, "  snapshot <id>    Capture point-in-time snapshot of a session\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -h, --help       Show help\n")
}
