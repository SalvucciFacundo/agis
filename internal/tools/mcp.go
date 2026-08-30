package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/mcp"
)

// MCPRunner bridges an MCP tool to the core.ToolRunner interface.
type MCPRunner struct {
	serverName string
	tool       mcp.Tool
	caller     func(ctx context.Context, toolName string, args any) (string, error)
}

// NewMCPRunner creates a new MCP tool runner backed by an mcp.Client.
func NewMCPRunner(serverName string, tool mcp.Tool, client mcp.Client) *MCPRunner {
	return &MCPRunner{
		serverName: serverName,
		tool:       tool,
		caller: func(ctx context.Context, toolName string, args any) (string, error) {
			if client == nil {
				return "", fmt.Errorf("mcp client for server %q is nil", serverName)
			}
			return client.CallTool(ctx, toolName, args)
		},
	}
}

// NewMCPRunnerWithCaller creates a new MCP tool runner with a custom caller function.
func NewMCPRunnerWithCaller(serverName string, tool mcp.Tool, caller func(ctx context.Context, toolName string, args any) (string, error)) *MCPRunner {
	return &MCPRunner{
		serverName: serverName,
		tool:       tool,
		caller:     caller,
	}
}

// Backend returns the backend identifier formatted as "mcp:<server_name>".
func (r *MCPRunner) Backend() string {
	return "mcp:" + r.serverName
}

// Name returns the namespaced tool identifier formatted as "mcp_<server_name>_<tool_name>".
func (r *MCPRunner) Name() string {
	return "mcp_" + r.serverName + "_" + r.tool.Name
}

// ToolName returns the raw MCP tool name exposed by the server.
func (r *MCPRunner) ToolName() string {
	return r.tool.Name
}

// Description returns the tool's description.
func (r *MCPRunner) Description() string {
	if r.tool.Description != "" {
		return r.tool.Description
	}
	return fmt.Sprintf("MCP tool %s from server %s", r.tool.Name, r.serverName)
}

// Tool returns the underlying MCP tool metadata.
func (r *MCPRunner) Tool() mcp.Tool {
	return r.tool
}

// ServerName returns the MCP server name.
func (r *MCPRunner) ServerName() string {
	return r.serverName
}

// Run executes the MCP tool with JSON arguments.
func (r *MCPRunner) Run(ctx context.Context, command string) (string, error) {
	trimmed := strings.TrimSpace(command)
	var args any
	if trimmed == "" || trimmed == "{}" {
		args = map[string]any{}
	} else {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "", fmt.Errorf("invalid json arguments for MCP tool %q: %w", r.tool.Name, err)
		}
		args = parsed
	}

	if r.caller == nil {
		return "", fmt.Errorf("no tool caller configured for %s", r.Name())
	}

	return r.caller(ctx, r.tool.Name, args)
}

// FromMCPManager extracts core.ToolRunner instances for all discovered tools across all active servers in the manager.
func FromMCPManager(mgr mcp.Manager) []core.ToolRunner {
	if mgr == nil {
		return nil
	}

	allTools := mgr.ListAllTools()
	if len(allTools) == 0 {
		return nil
	}

	var runners []core.ToolRunner
	for serverName, toolsList := range allTools {
		srv := serverName
		for _, t := range toolsList {
			tool := t
			runner := NewMCPRunnerWithCaller(srv, tool, func(ctx context.Context, toolName string, args any) (string, error) {
				return mgr.CallTool(ctx, srv, toolName, args)
			})
			runners = append(runners, runner)
		}
	}

	sort.Slice(runners, func(i, j int) bool {
		if runners[i].Backend() != runners[j].Backend() {
			return runners[i].Backend() < runners[j].Backend()
		}
		return runners[i].Name() < runners[j].Name()
	})

	return runners
}
