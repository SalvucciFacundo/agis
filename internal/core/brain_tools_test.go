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
		{ToolCall: &ToolCall{ID: id, Name: "shell", Arguments: `{"command":"git status"}`}},
	}
}

// fakeRunner records executed commands.
type fakeRunner struct {
	commands []string
	err      error
}

func (f *fakeRunner) Backend() string { return "local" }
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
		WithTools(runner, guard, nil),
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
		WithTools(&fakeRunner{}, &mapGuard{verdicts: map[string]Decision{}}, nil),
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
		WithTools(&fakeRunner{}, &mapGuard{verdicts: map[string]Decision{"git status": DecisionAsk}}, approver.Approve),
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
		WithTools(runner, &mapGuard{verdicts: map[string]Decision{"git status": DecisionAllow}}, nil),
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
	if brain.runner != nil || brain.guard != nil {
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
