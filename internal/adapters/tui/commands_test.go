package tui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/persona"
)

// evolutionRepo extends the tui fakeRepo with observable user-model rows.
type evolutionRepo struct {
	fakeRepo
	rows    []core.UserModel
	cleared bool
}

func (r *evolutionRepo) UserModelRows(context.Context, int) ([]core.UserModel, error) {
	return r.rows, nil
}

func (r *evolutionRepo) ClearUserModel(context.Context) error {
	r.cleared = true
	return nil
}

// newCommandModel wires a Model with overlays and an evolution layer so slash
// commands have real targets.
func newCommandModel(t *testing.T) (*Model, *persona.Evolution, *evolutionRepo) {
	t.Helper()
	repo := &evolutionRepo{rows: []core.UserModel{
		{Key: "user/pref/coffee", Value: "dark roast", Confidence: 0.8},
	}}
	evo := persona.NewEvolution(repo, slog.New(slog.DiscardHandler))
	stream := make(chan string, 8)
	brain := core.NewBrain(repo, &fakeProvider{}, core.WithSink(func(string) {}))
	m := New(brain, repo, stream,
		WithOverlays(persona.NewOverlays(map[string]string{"mentor": "Guide like a mentor."})),
		WithEvolution(evo),
	)
	return m, evo, repo
}

func sendCommand(m *Model, line string) *Model {
	m.input.SetValue(line)
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return model.(*Model)
}

func TestSlash_PersonalityAppliesOverlay(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/personality mentor")

	if !strings.Contains(m.history.String(), "personality: mentor") {
		t.Errorf("history = %q, want applied feedback", m.history.String())
	}
	if m.personality != "mentor" {
		t.Errorf("personality = %q, want mentor", m.personality)
	}
	if m.streaming {
		t.Error("streaming started for a slash command")
	}
}

func TestSlash_PersonalityNoneClears(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/personality teacher")
	m = sendCommand(m, "/personality none")

	if m.personality != "" {
		t.Errorf("personality = %q, want cleared", m.personality)
	}
	if !strings.Contains(m.history.String(), "cleared") {
		t.Errorf("history = %q, want cleared feedback", m.history.String())
	}
}

func TestSlash_PersonalityUnknownErrors(t *testing.T) {
	m, _, _ := newCommandModel(t)
	before := m.personality

	m = sendCommand(m, "/personality pirate")

	if !strings.Contains(m.history.String(), "unknown personality") {
		t.Errorf("history = %q, want unknown-personality feedback", m.history.String())
	}
	if m.personality != before {
		t.Errorf("personality changed to %q on unknown name", m.personality)
	}
}

func TestSlash_PersonaStatusShowsRows(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/persona status")

	got := m.history.String()
	if !strings.Contains(got, "evolution active (1 rows)") {
		t.Errorf("history = %q, want active status with row count", got)
	}
	if !strings.Contains(got, "personality none") {
		t.Errorf("history = %q, want personality none", got)
	}
}

func TestSlash_PersonaFreezeHidesLayer(t *testing.T) {
	m, evo, _ := newCommandModel(t)

	m = sendCommand(m, "/persona freeze")

	if !evo.Frozen() {
		t.Error("evolution not frozen after /persona freeze")
	}
	if !strings.Contains(m.history.String(), "frozen") {
		t.Errorf("history = %q, want frozen feedback", m.history.String())
	}
}

func TestSlash_PersonaResetClearsRows(t *testing.T) {
	m, _, repo := newCommandModel(t)

	m = sendCommand(m, "/persona reset")

	if !repo.cleared {
		t.Error("user model rows were not cleared")
	}
	if !strings.Contains(m.history.String(), "reset to seed state") {
		t.Errorf("history = %q, want reset feedback", m.history.String())
	}
}

func TestSlash_UnknownCommandErrors(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m.input.SetValue("/foo bar")
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	if cmd != nil {
		t.Error("cmd = non-nil for unknown command, want local handling only")
	}
	if !strings.Contains(m.history.String(), "unknown command: /foo") {
		t.Errorf("history = %q, want unknown-command feedback", m.history.String())
	}
}

func TestSlash_CommandsNeverPersistMessages(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/persona status")

	for line := range strings.SplitSeq(m.history.String(), "\n") {
		if strings.HasPrefix(line, userPrefix) && strings.Contains(line, "/persona") {
			t.Fatalf("slash command persisted as a user message: %q", line)
		}
	}
}

func TestSlash_Help(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/help")
	if !strings.Contains(m.history.String(), "Commands") || !strings.Contains(m.history.String(), "/profile") {
		t.Errorf("history does not contain /help overview: %s", m.history.String())
	}

	m = sendCommand(m, "/?")
	if !strings.Contains(m.history.String(), "/skills") {
		t.Errorf("history does not contain /? overview: %s", m.history.String())
	}
}

func TestSlash_Profile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGIS_HOME", home)
	if err := config.CreateProfile("work", ""); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	m, _, _ := newCommandModel(t)

	// Default profile display
	m = sendCommand(m, "/profile")
	if !strings.Contains(m.history.String(), "active profile:") {
		t.Errorf("expected active profile feedback, got: %s", m.history.String())
	}

	// Profile list
	m = sendCommand(m, "/profile list")
	if !strings.Contains(m.history.String(), "profiles:") {
		t.Errorf("expected profiles list feedback, got: %s", m.history.String())
	}

	// Profile switch
	m = sendCommand(m, "/profile use work")
	if !strings.Contains(m.history.String(), "switched to profile: work") {
		t.Errorf("expected switch feedback, got: %s", m.history.String())
	}
	if m.activeProfile != "work" {
		t.Errorf("m.activeProfile = %q, want 'work'", m.activeProfile)
	}
	if !strings.Contains(m.input.Prompt, "work") {
		t.Errorf("m.input.Prompt = %q, want to contain 'work'", m.input.Prompt)
	}
}

func TestSlash_Skills(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/skills")
	if !strings.Contains(m.history.String(), "skills:") {
		t.Errorf("expected skills feedback, got: %s", m.history.String())
	}
}

func TestSlash_Tools(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/tools")
	if !strings.Contains(m.history.String(), "tools:") {
		t.Errorf("expected tools feedback, got: %s", m.history.String())
	}
}

func TestSlash_MCP(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/mcp")
	if !strings.Contains(m.history.String(), "mcp:") {
		t.Errorf("expected mcp feedback, got: %s", m.history.String())
	}
}

func TestSlash_Doctor(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/doctor")
	if !strings.Contains(m.history.String(), "doctor diagnostics:") {
		t.Errorf("expected doctor feedback, got: %s", m.history.String())
	}
}

func TestSlash_Browser(t *testing.T) {
	m, _, _ := newCommandModel(t)

	m = sendCommand(m, "/browser")
	if !strings.Contains(m.history.String(), "browser:") {
		t.Errorf("expected browser feedback, got: %s", m.history.String())
	}
}
