package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// newApprovalModel wires a Model with approval channels.
func newApprovalModel(t *testing.T) (*Model, chan core.GuardRequest, chan core.Scope) {
	t.Helper()
	reqCh := make(chan core.GuardRequest, 1)
	respCh := make(chan core.Scope, 1)
	repo := &fakeRepo{}
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSink(func(string) {}))
	m := New(brain, repo, stream, WithApprovalChannels(reqCh, respCh))
	return m, reqCh, respCh
}

func deliverApproval(t *testing.T, m *Model, reqCh chan core.GuardRequest, subject string) {
	t.Helper()
	reqCh <- core.GuardRequest{Backend: "local", Category: "commands", Subject: subject}
	cmd := m.waitApproval()
	msg := cmd()
	if _, ok := msg.(approvalMsg); !ok {
		t.Fatalf("waitApproval msg = %T, want approvalMsg", msg)
	}
	m.Update(msg)
}

func TestApproval_PromptRendersSubject(t *testing.T) {
	m, reqCh, _ := newApprovalModel(t)
	deliverApproval(t, m, reqCh, "git push")

	if !strings.Contains(m.View(), "approve [local] git push") {
		t.Errorf("View() = %q, want the approval prompt with subject", m.View())
	}
}

func TestApproval_AllowOnceResolves(t *testing.T) {
	m, reqCh, respCh := newApprovalModel(t)
	deliverApproval(t, m, reqCh, "git push")

	m = sendCommand(m, "") // no-op to keep flow explicit
	resolve(t, m, "a")

	select {
	case got := <-respCh:
		if got != core.ScopeOnce {
			t.Errorf("scope = %v, want once", got)
		}
	default:
		t.Fatal("no scope delivered")
	}
	if m.pending != nil {
		t.Error("pending approval not cleared")
	}
	if !strings.Contains(m.history.String(), "approved (once)") {
		t.Errorf("history = %q, want approved feedback", m.history.String())
	}
}

func TestApproval_InterruptDenies(t *testing.T) {
	m, reqCh, respCh := newApprovalModel(t)
	deliverApproval(t, m, reqCh, "rm -rf /")

	// CtrlC on a visible prompt denies instead of quitting (safe default).
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = model.(*Model)

	select {
	case got := <-respCh:
		if got != core.ScopeDeny {
			t.Errorf("scope = %v, want deny on interrupt", got)
		}
	default:
		t.Fatal("no denial delivered")
	}
	if m.pending != nil {
		t.Error("pending not cleared after interrupt-deny")
	}
}

func TestApproval_SessionAndAlwaysKeys(t *testing.T) {
	scopes := map[string]core.Scope{"s": core.ScopeSession, "l": core.ScopeAlways, "n": core.ScopeDeny}
	for key, want := range scopes {
		m, reqCh, respCh := newApprovalModel(t)
		deliverApproval(t, m, reqCh, "cmd")

		m = resolve(t, m, key)

		got := <-respCh
		if got != want {
			t.Errorf("key %q resolved %v, want %v", key, got, want)
		}
	}
}

func resolve(t *testing.T, m *Model, key string) *Model {
	t.Helper()
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return model.(*Model)
}

func TestApproval_SecondAskStillReachesUI(t *testing.T) {
	m, reqCh, respCh := newApprovalModel(t)
	deliverApproval(t, m, reqCh, "first command")
	m = resolve(t, m, "a")
	<-respCh // drain the first resolution

	// A second ask must re-arm the watcher: deliver it through the same path.
	reqCh <- core.GuardRequest{Backend: "local", Category: "commands", Subject: "second command"}
	cmd := m.waitApproval()
	msg := cmd()
	if _, ok := msg.(approvalMsg); !ok {
		t.Fatalf("second ask msg = %T, want approvalMsg (watcher re-armed)", msg)
	}
	m.Update(msg)
	if !strings.Contains(m.View(), "second command") {
		t.Errorf("View() = %q, want the second prompt", m.View())
	}
	m = resolve(t, m, "n")
	if got := <-respCh; got != core.ScopeDeny {
		t.Errorf("second resolution = %v, want deny", got)
	}
}
