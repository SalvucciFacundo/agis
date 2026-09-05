package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/config"
	"gopkg.in/yaml.v3"
)

// RunConfigCLI routes and executes agis config subcommands.
func RunConfigCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printConfigUsage(stdout)
		return 0
	}

	subcmd := args[0]
	subArgs := args[1:]

	switch subcmd {
	case "help", "-h", "-help", "--help":
		printConfigUsage(stdout)
		return 0
	case "show":
		return runConfigShow(subArgs, stdout, stderr)
	case "get":
		return runConfigGet(subArgs, stdout, stderr)
	case "set":
		return runConfigSet(subArgs, stdout, stderr)
	case "path":
		return runConfigPath(subArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agis config: unknown subcommand '%s'\n\n", subcmd)
		printConfigUsage(stderr)
		return 2
	}
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis config <subcommand> [flags] [args]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  show                  Display current configuration\n")
	fmt.Fprintf(w, "  get <key>             Get the value of a configuration key\n")
	fmt.Fprintf(w, "  set <key> <value>     Update and persist a configuration key\n")
	fmt.Fprintf(w, "  path                  Print resolved configuration file path\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -config <path>        Path to configuration file\n")
	fmt.Fprintf(w, "  -json                 Output in JSON format (show, get)\n")
	fmt.Fprintf(w, "  -reveal               Display sensitive credentials in plaintext (show, get)\n")
}

func extractFlagsAndPositional(args []string) ([]string, []string) {
	var positional []string
	var flagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-config" || arg == "--config") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}

	return flagArgs, positional
}

func runConfigShow(args []string, stdout, stderr io.Writer) int {
	flagArgs, _ := extractFlagsAndPositional(args)

	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "", "path to config file")
	jsonOut := fs.Bool("json", false, "output in JSON format")
	reveal := fs.Bool("reveal", false, "reveal sensitive credential fields")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis config show [-config <path>] [-json] [-reveal]\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	cfg, err := config.Load(*configPath, config.WithWarnWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "agis config: loading config: %v\n", err)
		return 1
	}

	if !*reveal {
		cfg = config.MaskSecrets(cfg)
	}

	if *jsonOut {
		// Convert to map via YAML to preserve lowercase/snake_case tags in JSON
		yamlData, err := yaml.Marshal(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "agis config: marshaling yaml: %v\n", err)
			return 1
		}
		var rawMap any
		if err := yaml.Unmarshal(yamlData, &rawMap); err != nil {
			fmt.Fprintf(stderr, "agis config: unmarshaling map: %v\n", err)
			return 1
		}
		data, err := json.MarshalIndent(rawMap, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis config: marshaling json: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return 0
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "agis config: marshaling yaml: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, string(data))
	return 0
}

func runConfigGet(args []string, stdout, stderr io.Writer) int {
	flagArgs, posArgs := extractFlagsAndPositional(args)

	fs := flag.NewFlagSet("config get", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "", "path to config file")
	jsonOut := fs.Bool("json", false, "output in JSON format")
	reveal := fs.Bool("reveal", false, "reveal sensitive credentials")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis config get <key> [-config <path>] [-json] [-reveal]\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if len(posArgs) != 1 {
		fmt.Fprintf(stderr, "agis config get: requires exactly one configuration key\nUsage: agis config get <key> [-config <path>] [-json] [-reveal]\n")
		return 2
	}

	key := posArgs[0]
	cfg, err := config.Load(*configPath, config.WithWarnWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "agis config: loading config: %v\n", err)
		return 1
	}

	val, err := config.Get(cfg, key)
	if err != nil {
		fmt.Fprintf(stderr, "agis config: %v\n", err)
		return 1
	}

	if !*reveal && isSensitiveKey(key) {
		val = "[MASKED]"
	}

	if *jsonOut {
		data, err := json.MarshalIndent(val, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "agis config: marshaling json: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return 0
	}

	// For complex structures (slices, structs, maps), output YAML
	k := reflect.TypeOf(val).Kind()
	if k == reflect.Slice || k == reflect.Map || k == reflect.Struct {
		data, err := yaml.Marshal(val)
		if err != nil {
			fmt.Fprintf(stderr, "agis config: marshaling yaml: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "%v\n", val)
	return 0
}

func runConfigSet(args []string, stdout, stderr io.Writer) int {
	flagArgs, posArgs := extractFlagsAndPositional(args)

	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "", "path to config file")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis config set <key> <value> [-config <path>]\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if len(posArgs) != 2 {
		fmt.Fprintf(stderr, "agis config set: requires <key> and <value>\nUsage: agis config set <key> <value> [-config <path>]\n")
		return 2
	}

	key := posArgs[0]
	valStr := posArgs[1]

	effectivePath := config.ResolvePath(*configPath)
	cfg, err := config.Load(*configPath, config.WithWarnWriter(stderr))
	if err != nil {
		fmt.Fprintf(stderr, "agis config: loading config: %v\n", err)
		return 1
	}

	if err := config.Set(cfg, key, valStr); err != nil {
		fmt.Fprintf(stderr, "agis config: %v\n", err)
		return 1
	}

	if err := config.Save(effectivePath, cfg); err != nil {
		fmt.Fprintf(stderr, "agis config: saving config: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Updated '%s' to '%s'\n", key, valStr)
	return 0
}

func runConfigPath(args []string, stdout, stderr io.Writer) int {
	flagArgs, _ := extractFlagsAndPositional(args)

	fs := flag.NewFlagSet("config path", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "", "path to config file")

	fs.Usage = func() {
		fmt.Fprintf(stdout, "Usage: agis config path [-config <path>]\n")
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	resolved := config.ResolvePath(*configPath)
	fmt.Fprintf(stdout, "%s\n", resolved)
	return 0
}

func isSensitiveKey(key string) bool {
	norm := strings.ToLower(key)
	norm = strings.ReplaceAll(norm, "_", "")
	norm = strings.ReplaceAll(norm, "-", "")

	switch norm {
	case "llm.apikey", "gateway.telegram.token", "gateway.discord.token", "webhook.secret":
		return true
	default:
		return false
	}
}
