package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// scriptedToolProvider streams a fixed sequence of rounds: each round emits
// one tool call (or plain text once the script is exhausted).
type scriptedToolProvider struct {
	rounds [][]StreamEvent

	requests []ChatRequest
}

func (p *scriptedToolProvider) Chat(context.Context, ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not used")
}

func (p *scriptedToolProvider) Models() []ModelInfo { return nil }

func (p *scriptedToolProvider) Stream(_ context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	p.requests = append(p.requests, req)
	n := len(p.requests) - 1
	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		if n < len(p.rounds) {
			for _, ev := range p.rounds[n] {
				ch <- ev
			}
		} else {
			ch <- StreamEvent{Text: "final answer"}
		}
	}()
	return ch, nil
}

func toolCallEvents(id string) []StreamEvent {
	return []StreamEvent{
		{ToolCall: &ToolCall{ID: id, Name: "shell-local", Arguments: `{"command":"git status"}`}},
	}
}

// fakeRunner records executed commands.
type fakeRunner struct {
	name        string
	description string
	backend     string // defaults to "local"
	commands    []string
	err         error
}

func (f *fakeRunner) Name() string {
	if f.name != "" {
		return f.name
	}
	return "shell-" + f.Backend()
}

func (f *fakeRunner) Description() string {
	if f.description != "" {
		return f.description
	}
	return "Fake runner description for " + f.Backend()
}

func (f *fakeRunner) Backend() string {
	if f.backend == "" {
		return "local"
	}
	return f.backend
}
func (f *fakeRunner) Run(_ context.Context, command string) (string, error) {
	f.commands = append(f.commands, command)
	return "on branch main", f.err
}

// mapGuard decides from a static table and counts evaluations.
type mapGuard struct {
	verdicts map[string]Decision
	evals    int
}

func (m *mapGuard) Evaluate(_ context.Context, req GuardRequest) Decision {
	m.evals++
	if d, ok := m.verdicts[req.Subject]; ok {
		return d
	}
	return DecisionDeny
}

// scriptedApprover returns queued scopes.
type scriptedApprover struct {
	scopes []Scope
	got    []GuardRequest
}

func (a *scriptedApprover) Approve(_ context.Context, req GuardRequest) Scope {
	a.got = append(a.got, req)
	if len(a.scopes) == 0 {
		return ScopeDeny
	}
	s := a.scopes[0]
	a.scopes = a.scopes[1:]
	return s
}

func TestBrainLoop_AllowedToolFeedsResultBack(t *testing.T) {
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		toolCallEvents("call_1"),
		{{Text: "branch main"}},
	}}
	repo := newFakeRepo()
	runner := &fakeRunner{}
	guard := &mapGuard{verdicts: map[string]Decision{"git status": DecisionAllow}}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{runner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "check repo"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(runner.commands) != 1 || runner.commands[0] != "git status" {
		t.Errorf("executed = %v, want [git status]", runner.commands)
	}

	// Round 2 request carries assistant tool request + RoleTool feedback.
	msgs := provider.requests[1].Messages
	var sawAssistantWithCalls, sawToolResult bool
	for _, m := range msgs {
		if m.Role == RoleAssistant && len(m.ToolCalls) == 1 && m.ToolCalls[0].ID == "call_1" {
			sawAssistantWithCalls = true
		}
		if m.Role == RoleTool && m.ToolCallID == "call_1" && strings.Contains(m.Content, "on branch main") {
			sawToolResult = true
		}
	}
	if !sawAssistantWithCalls || !sawToolResult {
		t.Errorf("feedback protocol incomplete: assistant=%v tool=%v", sawAssistantWithCalls, sawToolResult)
	}
}

func TestBrainLoop_DeniedToolInformsModel(t *testing.T) {
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		toolCallEvents("call_deny"),
		{{Text: "understood"}},
	}}
	repo := newFakeRepo()
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{&fakeRunner{}}, &mapGuard{verdicts: map[string]Decision{}}, nil),
	)

	if err := brain.Step(context.Background(), "do it"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs := provider.requests[1].Messages
	found := false
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID == "call_deny" && strings.Contains(m.Content, "blocked by policy") {
			found = true
		}
	}
	if !found {
		t.Error("denied call did not inform the model")
	}
}

func TestBrainLoop_AskApprovedOnceRuns(t *testing.T) {
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		toolCallEvents("call_ask"),
		{{Text: "done"}},
	}}
	approver := &scriptedApprover{scopes: []Scope{ScopeOnce}}
	brain := NewBrain(newFakeRepo(), provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{&fakeRunner{}}, &mapGuard{verdicts: map[string]Decision{"git status": DecisionAsk}}, approver.Approve),
	)

	if err := brain.Step(context.Background(), "go"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if len(approver.got) != 1 || approver.got[0].Subject != "git status" {
		t.Errorf("approver got %+v, want one git status request", approver.got)
	}
}

func TestBrainLoop_CapStopsRunaway(t *testing.T) {
	calls := toolCallEvents("call_x")
	rounds := make([][]StreamEvent, maxToolRounds+2)
	for i := range rounds {
		rounds[i] = calls
	}
	provider := &scriptedToolProvider{rounds: rounds}
	repo := newFakeRepo()
	runner := &fakeRunner{}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{runner}, &mapGuard{verdicts: map[string]Decision{"git status": DecisionAllow}}, nil),
	)

	if err := brain.Step(context.Background(), "loop"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(runner.commands) > maxToolRounds {
		t.Errorf("executed %d commands, want <= %d (cap)", len(runner.commands), maxToolRounds)
	}
	// The cap notice reached the model on the final round.
	last := provider.requests[len(provider.requests)-1]
	foundNotice := false
	for _, m := range last.Messages {
		if strings.Contains(m.Content, "Tool round limit reached") {
			foundNotice = true
		}
	}
	if !foundNotice {
		t.Error("cap notice never sent to the model")
	}
	// The cap itself was audited.
	capped := false
	for _, e := range repo.auditEntries {
		if e.Scope == "round-cap" {
			capped = true
		}
	}
	if !capped {
		t.Error("round cap was not audited")
	}
}

func TestBrainLoop_ToolsDisabledStreamsUnchanged(t *testing.T) {
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		toolCallEvents("call_z"),
	}}
	brain := NewBrain(newFakeRepo(), provider, WithSink(func(string) {}))

	// No WithTools: the tool call event is ignored entirely.
	if err := brain.Step(context.Background(), "hi"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if len(provider.requests) != 1 {
		t.Errorf("requests = %d, want 1 (no tool round entered)", len(provider.requests))
	}
	if len(brain.runners) != 0 || brain.guard != nil {
		t.Error("tools wired without WithTools")
	}
}

func TestCommandFromArgs(t *testing.T) {
	cmd, err := commandFromArgs(`{"command":"git status"}`)
	if err != nil || cmd != "git status" {
		t.Errorf("commandFromArgs = %q, %v; want git status", cmd, err)
	}
	if _, err := commandFromArgs(`not json`); err == nil {
		t.Error("malformed args accepted")
	}
	if _, err := commandFromArgs(`{"command":"  "}`); err == nil {
		t.Error("empty command accepted")
	}
}

func TestBrainLoop_CapFinalAnswerStillStreams(t *testing.T) {
	calls := toolCallEvents("call_x")
	rounds := make([][]StreamEvent, maxToolRounds+1)
	for i := range rounds {
		rounds[i] = calls
	}
	rounds[maxToolRounds] = []StreamEvent{{Text: "final visible answer"}}
	provider := &scriptedToolProvider{rounds: rounds}

	var streamed strings.Builder
	brain := NewBrain(newFakeRepo(), provider,
		WithSink(func(s string) { streamed.WriteString(s) }),
		WithTools([]ToolRunner{&fakeRunner{}}, &mapGuard{verdicts: map[string]Decision{"git status": DecisionAllow}}, nil),
	)

	if err := brain.Step(context.Background(), "loop"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if !strings.Contains(streamed.String(), "final visible answer") {
		t.Errorf("sink = %q, want the forced final answer streamed live", streamed.String())
	}
}

func TestBrainLoop_RoutesByBackendToolName(t *testing.T) {
	local := &fakeRunner{}
	docker := &fakeRunner{backend: "docker"}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "c1", Name: "shell-docker", Arguments: `{"command":"echo in container"}`}},
		},
		{{Text: "done"}},
	}}
	guard := &mapGuard{verdicts: map[string]Decision{"echo in container": DecisionAllow}}
	brain := NewBrain(newFakeRepo(), provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{local, docker}, guard, nil),
	)

	if err := brain.Step(context.Background(), "run it"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(local.commands) != 0 {
		t.Errorf("local executed %v, want untouched", local.commands)
	}
	if len(docker.commands) != 1 || docker.commands[0] != "echo in container" {
		t.Errorf("docker executed = %v, want the call routed to shell-docker", docker.commands)
	}
}

// fakeMCPRunner simulates an MCP tool runner.
type fakeMCPRunner struct {
	serverName string
	toolName   string
	executed   []string
	out        string
	err        error
}

func (m *fakeMCPRunner) Backend() string     { return "mcp:" + m.serverName }
func (m *fakeMCPRunner) Name() string        { return "mcp_" + m.serverName + "_" + m.toolName }
func (m *fakeMCPRunner) ToolName() string    { return m.toolName }
func (m *fakeMCPRunner) Description() string { return "MCP tool " + m.toolName }
func (m *fakeMCPRunner) Run(_ context.Context, args string) (string, error) {
	m.executed = append(m.executed, args)
	return m.out, m.err
}

func TestBrainLoop_MCPTool_Allowed(t *testing.T) {
	mcpRunner := &fakeMCPRunner{
		serverName: "github",
		toolName:   "create_issue",
		out:        `{"issue_id": 123}`,
	}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_mcp", Name: "mcp_github_create_issue", Arguments: `{"title":"bug report"}`}},
		},
		{{Text: "issue created"}},
	}}
	repo := newFakeRepo()
	guard := &mapGuard{verdicts: map[string]Decision{"create_issue": DecisionAllow}}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{mcpRunner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "create issue"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(mcpRunner.executed) != 1 || mcpRunner.executed[0] != `{"title":"bug report"}` {
		t.Errorf("mcpRunner executed = %v, want [%s]", mcpRunner.executed, `{"title":"bug report"}`)
	}

	// Verify feedback protocol to LLM
	msgs := provider.requests[1].Messages
	var sawToolResult bool
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID == "call_mcp" && strings.Contains(m.Content, `{"issue_id": 123}`) {
			sawToolResult = true
		}
	}
	if !sawToolResult {
		t.Error("MCP tool execution result was not fed back as RoleTool")
	}
}

func TestBrainLoop_MCPTool_SandboxDenied(t *testing.T) {
	mcpRunner := &fakeMCPRunner{
		serverName: "github",
		toolName:   "create_issue",
		out:        `{"issue_id": 123}`,
	}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_mcp_deny", Name: "mcp_github_create_issue", Arguments: `{"title":"bug"}`}},
		},
		{{Text: "understood"}},
	}}
	repo := newFakeRepo()
	guard := &mapGuard{verdicts: map[string]Decision{}} // default Deny
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{mcpRunner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "create issue"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(mcpRunner.executed) != 0 {
		t.Errorf("mcpRunner executed = %v, want none (blocked by policy)", mcpRunner.executed)
	}

	msgs := provider.requests[1].Messages
	var foundBlocked bool
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID == "call_mcp_deny" && strings.Contains(m.Content, "blocked by policy") {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Error("denied MCP tool call did not inform model with blocked by policy")
	}
}

func TestBrainLoop_MCPTool_AskWithAutoDenyApprover(t *testing.T) {
	mcpRunner := &fakeMCPRunner{
		serverName: "filesystem",
		toolName:   "delete_file",
		out:        `deleted`,
	}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_mcp_ask", Name: "mcp_filesystem_delete_file", Arguments: `{"path":"/tmp/test"}`}},
		},
		{{Text: "understood"}},
	}}
	repo := newFakeRepo()
	guard := &mapGuard{verdicts: map[string]Decision{"delete_file": DecisionAsk}}
	approver := &scriptedApprover{scopes: []Scope{ScopeDeny}} // AutoDenyApprover yields ScopeDeny
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{mcpRunner}, guard, approver.Approve),
	)

	if err := brain.Step(context.Background(), "delete file"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(mcpRunner.executed) != 0 {
		t.Errorf("mcpRunner executed = %v, want none (auto-denied)", mcpRunner.executed)
	}

	if len(approver.got) != 1 || approver.got[0].Backend != "mcp:filesystem" || approver.got[0].Subject != "delete_file" {
		t.Errorf("approver got %+v, want GuardRequest for mcp:filesystem delete_file", approver.got)
	}
}

type fakeWebRunner struct {
	name     string
	executed []string
	out      string
}

func (f *fakeWebRunner) Name() string        { return f.name }
func (f *fakeWebRunner) Description() string { return "fake web tool" }
func (f *fakeWebRunner) Backend() string     { return "web" }
func (f *fakeWebRunner) Run(_ context.Context, command string) (string, error) {
	f.executed = append(f.executed, command)
	return f.out, nil
}

func TestBrainLoop_WebTools_EvaluationAndExecution(t *testing.T) {
	webRunner := &fakeWebRunner{
		name: "web_search",
		out:  `[{"title":"Go","url":"https://go.dev","snippet":"The Go Programming Language"}]`,
	}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_web_1", Name: "web_search", Arguments: `{"query":"golang"}`}},
		},
		{{Text: "here are the results"}},
	}}
	repo := newFakeRepo()
	guard := &mapGuard{verdicts: map[string]Decision{"golang": DecisionAllow}}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{webRunner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "search golang"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(webRunner.executed) != 1 || webRunner.executed[0] != `{"query":"golang"}` {
		t.Errorf("webRunner executed = %v, want raw arguments", webRunner.executed)
	}

	msgs := provider.requests[1].Messages
	var foundOutput bool
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID == "call_web_1" && strings.Contains(m.Content, "https://go.dev") {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Error("model request missing web_search output")
	}
}

type fakeSubagentRunner struct {
	name     string
	executed []string
	out      string
}

func (f *fakeSubagentRunner) Name() string        { return f.name }
func (f *fakeSubagentRunner) Description() string { return "fake subagent runner" }
func (f *fakeSubagentRunner) Backend() string     { return "subagent" }
func (f *fakeSubagentRunner) Run(_ context.Context, command string) (string, error) {
	f.executed = append(f.executed, command)
	return f.out, nil
}

type capturingGuard struct {
	requests []GuardRequest
	verdict  Decision
}

func (c *capturingGuard) Evaluate(_ context.Context, req GuardRequest) Decision {
	c.requests = append(c.requests, req)
	if c.verdict != 0 {
		return c.verdict
	}
	return DecisionAllow
}

func TestBrainLoop_SubagentTool_EvaluationAndExecution(t *testing.T) {
	subagentRunner := &fakeSubagentRunner{
		name: "delegate_task",
		out:  "subagent task completed successfully",
	}
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_sub_1", Name: "delegate_task", Arguments: `{"task":"analyze logs","context":"log data","max_turns":5}`}},
		},
		{{Text: "task finished"}},
	}}
	repo := newFakeRepo()
	guard := &capturingGuard{verdict: DecisionAllow}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{subagentRunner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "delegate log analysis"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(guard.requests) != 1 {
		t.Fatalf("guard evaluated %d times, want 1", len(guard.requests))
	}
	req := guard.requests[0]
	if req.Backend != "subagent" {
		t.Errorf("guard request Backend = %q, want %q", req.Backend, "subagent")
	}
	if req.Category != CategoryExecution {
		t.Errorf("guard request Category = %q, want %q", req.Category, CategoryExecution)
	}
	if req.Subject != "analyze logs" {
		t.Errorf("guard request Subject = %q, want %q", req.Subject, "analyze logs")
	}

	if len(subagentRunner.executed) != 1 || subagentRunner.executed[0] != `{"task":"analyze logs","context":"log data","max_turns":5}` {
		t.Errorf("subagentRunner executed = %v", subagentRunner.executed)
	}

	msgs := provider.requests[1].Messages
	var foundOutput bool
	for _, m := range msgs {
		if m.Role == RoleTool && m.ToolCallID == "call_sub_1" && strings.Contains(m.Content, "subagent task completed successfully") {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Error("model request missing delegate_task output")
	}
}

func TestBrainLoop_SubagentTool_LongTaskTruncationInGuard(t *testing.T) {
	subagentRunner := &fakeSubagentRunner{
		name: "delegate_task",
		out:  "task completed",
	}
	longTask := strings.Repeat("A", 300)
	provider := &scriptedToolProvider{rounds: [][]StreamEvent{
		{
			{ToolCall: &ToolCall{ID: "call_sub_long", Name: "delegate_task", Arguments: `{"task":"` + longTask + `"}`}},
		},
		{{Text: "done"}},
	}}
	repo := newFakeRepo()
	guard := &capturingGuard{verdict: DecisionAllow}
	brain := NewBrain(repo, provider,
		WithSink(func(string) {}),
		WithTools([]ToolRunner{subagentRunner}, guard, nil),
	)

	if err := brain.Step(context.Background(), "delegate long task"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if len(guard.requests) != 1 {
		t.Fatalf("guard evaluated %d times, want 1", len(guard.requests))
	}
	req := guard.requests[0]
	if len(req.Subject) != 256 {
		t.Errorf("guard request Subject len = %d, want 256", len(req.Subject))
	}
	if req.Subject != strings.Repeat("A", 256) {
		t.Errorf("guard request Subject truncated incorrectly")
	}
}


