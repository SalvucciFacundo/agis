package tui

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbletea"

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
