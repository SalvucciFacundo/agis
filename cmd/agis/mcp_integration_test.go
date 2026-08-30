package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/mcp"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/tools"
	"go.uber.org/goleak"
)

// createStdioMCPServerScript creates an executable mock MCP stdio server.
func createStdioMCPServerScript(t *testing.T, dir string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, "mcp_server.sh")
	content := `#!/bin/sh
while IFS= read -r line; do
  id=$(echo "$line" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
  if [ -z "$id" ]; then
    id=$(echo "$line" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
  fi
  if [ -z "$id" ]; then
    id=1
  fi
  case "$line" in
    *'"method":"initialize"'*)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{\"tools\":{}},\"serverInfo\":{\"name\":\"test-server\",\"version\":\"1.0.0\"}}}"
      ;;
    *'"method":"tools/list"'*)
      echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"tools\":[{\"name\":\"echo\",\"description\":\"Echoes input text\",\"inputSchema\":{\"type\":\"object\"}},{\"name\":\"dangerous_delete\",\"description\":\"Destructive tool\",\"inputSchema\":{\"type\":\"object\"}}]}}"
      ;;
    *'"method":"tools/call"'*)
      if echo "$line" | grep -q 'dangerous_delete'; then
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"file deleted\"}],\"isError\":false}}"
      else
        echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"MCP echo response: OK\"}],\"isError\":false}}"
      fi
      ;;
  esac
done
`
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatalf("writing mock mcp server script: %v", err)
	}
	return scriptPath
}

func TestMCP_EndToEnd_BrainAndPolicyGuard(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	tmpDir := t.TempDir()
	serverScript := createStdioMCPServerScript(t, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Setup MCP Manager
	mcpCfg := config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"mock-srv": {
				Command: serverScript,
			},
		},
	}

	mgr := mcp.NewManager(mcpCfg)
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("mgr.Start() error = %v", err)
	}
	defer func() { _ = mgr.Stop() }()

	runners := tools.FromMCPManager(mgr)
	if len(runners) != 2 {
		t.Fatalf("FromMCPManager returned %d runners, want 2", len(runners))
	}

	// 2. Setup SQLite Repository for conversation and audit log
	repo, err := memory.NewRepository(ctx, ":memory:")
	if err != nil {
		t.Fatalf("memory.NewRepository() error = %v", err)
	}
	defer repo.Close()

	// 3. Setup Policy Guard with rules:
	// "mock-srv" tier sandbox: allows "echo" via rule, denies "dangerous_delete"
	policyYAML := `
tiers:
  mcp:mock-srv: sandbox
rules:
  commands:
    - backend: "mcp:mock-srv"
      pattern: "echo"
      action: "allow"
`
	policyFile := filepath.Join(tmpDir, "policy.yaml")
	if err := os.WriteFile(policyFile, []byte(policyYAML), 0o600); err != nil {
		t.Fatalf("write policy.yaml: %v", err)
	}

	pstore, err := policy.Load(policyFile)
	if err != nil {
		t.Fatalf("policy.Load() error = %v", err)
	}
	pstore.SetAuditSink(repo)

	// 4. Setup Mock Provider for LLM tool call simulation
	var currentRound int
	provider := &mockEchoProvider{
		toolFn: func(req core.ChatRequest) (<-chan core.StreamEvent, error) {
			ch := make(chan core.StreamEvent, 2)
			go func() {
				defer close(ch)
				currentRound++
				if currentRound == 1 {
					// Round 1: Model calls allowed tool mcp_mock-srv_echo
					ch <- core.StreamEvent{
						ToolCall: &core.ToolCall{
							ID:        "call_1",
							Name:      "mcp_mock-srv_echo",
							Arguments: `{"text":"test"}`},
					}
				} else {
					// Round 2: Model answers with final response
					ch <- core.StreamEvent{Text: "Task completed with tool output"}
				}
			}()
			return ch, nil
		},
	}

	brain := core.NewBrain(
		repo,
		provider,
		core.WithTools(runners, pstore, gateway.NewAutoDenyApprover(nil)),
	)

	// --- Step 1: Execute allowed MCP tool call ---
	if err := brain.Step(ctx, "run echo tool"); err != nil {
		t.Fatalf("brain.Step() error = %v", err)
	}

	// Verify provider received the tool call result in subsequent turn
	provider.mu.Lock()
	callsStep1 := make([]core.ChatRequest, len(provider.calls))
	copy(callsStep1, provider.calls)
	provider.mu.Unlock()

	if len(callsStep1) < 2 {
		t.Fatalf("provider received %d requests, want at least 2", len(callsStep1))
	}

	var sawToolOutput bool
	for _, m := range callsStep1[1].Messages {
		if m.Role == core.RoleTool && strings.Contains(m.Content, "MCP echo response: OK") {
			sawToolOutput = true
		}
	}
	if !sawToolOutput {
		t.Errorf("round 2 messages do not contain tool output, msgs = %+v", callsStep1[1].Messages)
	}

	// --- Step 2: Test unapproved MCP tool call with AutoDenyApprover ---
	provider.mu.Lock()
	provider.calls = nil
	currentRound = 0
	provider.toolFn = func(req core.ChatRequest) (<-chan core.StreamEvent, error) {
		ch := make(chan core.StreamEvent, 2)
		go func() {
			defer close(ch)
			currentRound++
			if currentRound == 1 {
				// Round 1: Model calls unapproved tool mcp_mock-srv_dangerous_delete
				ch <- core.StreamEvent{
					ToolCall: &core.ToolCall{
						ID:        "call_deny",
						Name:      "mcp_mock-srv_dangerous_delete",
						Arguments: `{"target":"/file.txt"}`},
				}
			} else {
				ch <- core.StreamEvent{Text: "Handled denial"}
			}
		}()
		return ch, nil
	}
	provider.mu.Unlock()

	if err := brain.Step(ctx, "delete file"); err != nil {
		t.Fatalf("brain.Step() for denied tool error = %v", err)
	}

	// Verify denied tool result fed back to model
	provider.mu.Lock()
	callsStep2 := make([]core.ChatRequest, len(provider.calls))
	copy(callsStep2, provider.calls)
	provider.mu.Unlock()

	if len(callsStep2) < 2 {
		t.Fatalf("provider received %d requests in step 2, want at least 2", len(callsStep2))
	}

	var sawDeniedFeedback bool
	for _, m := range callsStep2[1].Messages {
		if m.Role == core.RoleTool && m.ToolCallID == "call_deny" && strings.Contains(m.Content, "blocked by policy") {
			sawDeniedFeedback = true
		}
	}
	if !sawDeniedFeedback {
		t.Errorf("conversation missing blocked by policy feedback for denied tool, msgs = %+v", callsStep2[1].Messages)
	}

	// Verify audit log captured the decision
	audits, err := repo.AuditTail(ctx, 10)
	if err != nil {
		t.Fatalf("AuditTail() error = %v", err)
	}
	if len(audits) == 0 {
		t.Error("expected audit entries in audit log, got 0")
	}
}

func TestMCP_EndToEnd_CLISubcommands(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	tmpDir := t.TempDir()
	serverScript := createStdioMCPServerScript(t, tmpDir)

	configYAML := fmt.Sprintf(`
mcp:
  enabled: true
  servers:
    mock-srv:
      command: "%s"
    disabled-srv:
      command: "nonexistent"
      disabled: true
`, serverScript)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	// 1. Test `agis mcp list`
	var listOut, listErr bytes.Buffer
	code := RunMCPCLI([]string{"list", "-config", cfgPath}, &listOut, &listErr)
	if code != 0 {
		t.Fatalf("agis mcp list exit code = %d (stderr: %s)", code, listErr.String())
	}
	outStr := listOut.String()
	if !strings.Contains(outStr, "mock-srv") || !strings.Contains(outStr, "online") {
		t.Errorf("list output missing online mock-srv: %s", outStr)
	}
	if !strings.Contains(outStr, "echo") || !strings.Contains(outStr, "dangerous_delete") {
		t.Errorf("list output missing discovered tools: %s", outStr)
	}
	if !strings.Contains(outStr, "disabled-srv") || !strings.Contains(outStr, "disabled") {
		t.Errorf("list output missing disabled-srv status: %s", outStr)
	}

	// 2. Test `agis mcp test mock-srv echo`
	var testOut, testErr bytes.Buffer
	code = RunMCPCLI([]string{"test", "mock-srv", "echo", `{"text":"hello"}`, "-config", cfgPath}, &testOut, &testErr)
	if code != 0 {
		t.Fatalf("agis mcp test exit code = %d (stderr: %s)", code, testErr.String())
	}
	testStr := testOut.String()
	if !strings.Contains(testStr, "MCP echo response: OK") {
		t.Errorf("test output missing expected response, got: %s", testStr)
	}
	if !strings.Contains(testStr, "Execution time:") {
		t.Errorf("test output missing execution time, got: %s", testStr)
	}
}
