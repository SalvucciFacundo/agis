package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestWriteRegistry_ListsSkills(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill-registry.md")

	if err := WriteRegistry(path, []core.Skill{
		{Name: "deploy-notes", Trigger: "deploy", Source: core.SourceAgent, UsageCount: 4, Description: "ship a release"},
		{Name: "imported-thing", Source: core.SourceImported, Description: "from a file"},
	}); err != nil {
		t.Fatalf("WriteRegistry() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{"deploy-notes", "agent", "imported", "| 4 |", "ship a release"} {
		if !strings.Contains(got, want) {
			t.Errorf("registry missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "release-checklist") {
		t.Error("registry contains a skill that was not passed")
	}
	if !strings.HasPrefix(got, "# AGIS Skill Registry") || !strings.Contains(got, "Last updated:") {
		t.Errorf("registry missing header:\n%s", got)
	}
}

func TestWriteRegistry_EmptyIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.md")
	if err := WriteRegistry(path, nil); err != nil {
		t.Fatalf("WriteRegistry(nil) error = %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "| Name |") {
		t.Errorf("registry = %q, want header with empty table", data)
	}
}

func TestWriteRegistry_NoTmpLeftBehind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.md")
	if err := WriteRegistry(path, nil); err != nil {
		t.Fatalf("WriteRegistry() error = %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("tmp file survived: %v", err)
	}
}

func TestHub_SyncRegistryWarnsOnly(t *testing.T) {
	hub := NewHub(&fakeSkillRepo{}, discardLogger())
	hub.Add(core.Skill{Name: "a", Description: "d"})

	// Unwritable directory must not panic or fail: the error is swallowed.
	hub.SyncRegistry("/nonexistent-dir-xyz/reg.md")
}
