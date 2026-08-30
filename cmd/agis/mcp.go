package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/mcp"
	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
)

// RunMCPCLI routes the `agis mcp` subcommands.
func RunMCPCLI(args []string, stdout, stderr io.Writer) int {
	subcommand := "list"
	var positional []string
	var flagArgs []string

	if len(args) > 0 {
		switch args[0] {
		case "list":
			subcommand = "list"
			flagArgs = args[1:]
		case "test":
			subcommand = "test"
			// Split positional arguments from flags
			for i := 1; i < len(args); i++ {
				if strings.HasPrefix(args[i], "-") {
					flagArgs = args[i:]
					break
				}
				positional = append(positional, args[i])
			}
		case "-h", "--help", "-help":
			printMCPUsage(stdout)
			return 0
		default:
			if strings.HasPrefix(args[0], "-") {
				flagArgs = args
			} else {
				fmt.Fprintf(stderr, "agis mcp: unknown subcommand %q\n", args[0])
				printMCPUsage(stderr)
				return 2
			}
		}
	}

	fs := flag.NewFlagSet("mcp "+subcommand, flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "", "path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)")

	fs.Usage = func() {
		printMCPUsage(stdout)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "agis mcp: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "agis mcp: loading config: %v\n", err)
		return 1
	}

	switch subcommand {
	case "list":
		return runMCPList(cfg.MCP, stdout, stderr)
	case "test":
		if len(positional) < 2 {
			fmt.Fprintf(stderr, "agis mcp test: server and tool arguments are required (e.g. agis mcp test <server> <tool> [args])\n")
			return 2
		}
		targetServer := positional[0]
		targetTool := positional[1]
		var jsonArgs string
		if len(positional) > 2 {
			jsonArgs = positional[2]
		}
		return runMCPTest(cfg.MCP, targetServer, targetTool, jsonArgs, stdout, stderr)
	default:
		printMCPUsage(stdout)
		return 2
	}
}

func printMCPUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: agis mcp [list|test] [args] [flags]\n\n")
	fmt.Fprintf(w, "Subcommands:\n")
	fmt.Fprintf(w, "  list                                  List all configured MCP servers and their tools (default)\n")
	fmt.Fprintf(w, "  test <server> <tool> [json_args]      Directly invoke an MCP tool without LLM orchestration\n\n")
	fmt.Fprintf(w, "Flags:\n")
	fmt.Fprintf(w, "  -config string\n")
	fmt.Fprintf(w, "        path to config file (default: $AGIS_HOME/config.yaml or ~/.agis/config.yaml)\n")
}

func runMCPList(cfg config.MCPConfig, stdout, stderr io.Writer) int {
	if len(cfg.Servers) == 0 {
		fmt.Fprintf(stdout, "No MCP servers configured in config.yaml\n")
		return 0
	}

	serverNames := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	fmt.Fprintf(stdout, "Configured MCP Servers (%d):\n\n", len(serverNames))

	hasError := false
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, name := range serverNames {
		sCfg := cfg.Servers[name]
		transportType := "stdio"
		if sCfg.URL != "" {
			transportType = "sse"
		}

		if sCfg.Disabled {
			fmt.Fprintf(stdout, "  • %-20s [%s] [disabled]\n", name, transportType)
			continue
		}

		client, err := buildClientForServer(name, sCfg)
		if err != nil {
			fmt.Fprintf(stdout, "  • %-20s [%s] [offline] (config error: %v)\n", name, transportType, err)
			hasError = true
			continue
		}

		if err := client.Initialize(ctx); err != nil {
			fmt.Fprintf(stdout, "  • %-20s [%s] [offline] (init failed: %v)\n", name, transportType, err)
			_ = client.Close()
			hasError = true
			continue
		}

		tools, err := client.ListTools(ctx)
		_ = client.Close()
		if err != nil {
			fmt.Fprintf(stdout, "  • %-20s [%s] [offline] (tools/list failed: %v)\n", name, transportType, err)
			hasError = true
			continue
		}

		fmt.Fprintf(stdout, "  • %-20s [%s] [online] - %d tool(s) discovered:\n", name, transportType, len(tools))
		for _, t := range tools {
			desc := t.Description
			if desc != "" {
				desc = " - " + desc
			}
			fmt.Fprintf(stdout, "      - %-20s%s\n", t.Name, desc)
		}
	}

	if hasError {
		return 1
	}
	return 0
}

func runMCPTest(cfg config.MCPConfig, serverName, toolName, jsonArgs string, stdout, stderr io.Writer) int {
	sCfg, ok := cfg.Servers[serverName]
	if !ok {
		fmt.Fprintf(stderr, "agis mcp test: server %q not found in configuration\n", serverName)
		return 1
	}
	if sCfg.Disabled {
		fmt.Fprintf(stderr, "agis mcp test: server %q is disabled\n", serverName)
		return 1
	}

	var parsedArgs any
	trimmed := strings.TrimSpace(jsonArgs)
	if trimmed == "" || trimmed == "{}" {
		parsedArgs = map[string]any{}
	} else {
		var argsMap map[string]any
		if err := json.Unmarshal([]byte(trimmed), &argsMap); err != nil {
			fmt.Fprintf(stderr, "agis mcp test: invalid JSON arguments: %v\n", err)
			return 2
		}
		parsedArgs = argsMap
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := buildClientForServer(serverName, sCfg)
	if err != nil {
		fmt.Fprintf(stderr, "agis mcp test: creating client for %q: %v\n", serverName, err)
		return 1
	}
	defer client.Close()

	if err := client.Initialize(ctx); err != nil {
		fmt.Fprintf(stderr, "agis mcp test: handshake with %q failed: %v\n", serverName, err)
		return 1
	}

	start := time.Now()
	result, err := client.CallTool(ctx, toolName, parsedArgs)
	duration := time.Since(start)

	if err != nil {
		fmt.Fprintf(stderr, "agis mcp test error: %v (took %v)\n", err, duration)
		return 1
	}

	fmt.Fprintf(stdout, "Tool: %s (server: %s)\n", toolName, serverName)
	fmt.Fprintf(stdout, "Execution time: %v\n\n", duration)
	fmt.Fprintf(stdout, "Result:\n%s\n", result)
	return 0
}

func buildClientForServer(serverName string, sCfg config.MCPServerConfig) (mcp.Client, error) {
	var tr transport.Transport
	var err error

	if sCfg.Command != "" {
		tr, err = transport.NewStdio(transport.StdioConfig{
			Command: sCfg.Command,
			Args:    sCfg.Args,
			Env:     sCfg.Env,
		})
	} else if sCfg.URL != "" {
		tr, err = transport.NewSSE(transport.SSEConfig{
			URL: sCfg.URL,
		})
	} else {
		return nil, fmt.Errorf("server %q must configure either command or url", serverName)
	}

	if err != nil {
		return nil, err
	}
	return mcp.NewClient(tr), nil
}
