package skills

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// fakeSkillRepo is a stateful core.Repository double for hub tests. Only the
// skill methods carry behavior; the rest are inert.
type fakeSkillRepo struct {
	store    []core.Skill
	failSave error
}

func (r *fakeSkillRepo) SaveSkill(_ context.Context, s core.Skill) error {
	if r.failSave != nil {
		return r.failSave
	}
	for i := range r.store {
		if r.store[i].Name == s.Name {
			r.store[i] = s
			return nil
		}
	}
	r.store = append(r.store, s)
	return nil
}

func (r *fakeSkillRepo) ListSkills(context.Context) ([]core.Skill, error) {
	out := make([]core.Skill, len(r.store))
	copy(out, r.store)
	return out, nil
}

func (r *fakeSkillRepo) CreateConversation(context.Context, string) (*core.Conversation, error) {
	return nil, errors.New("not used")
}
func (r *fakeSkillRepo) LatestConversation(context.Context) (*core.Conversation, error) {
	return nil, core.ErrNotFound
}
func (r *fakeSkillRepo) AppendMessage(context.Context, string, core.Message) error {
	return nil
}
func (r *fakeSkillRepo) Messages(context.Context, string, int) ([]core.Message, error) {
	return nil, nil
}
func (r *fakeSkillRepo) Search(context.Context, string, int) ([]core.SearchResult, error) {
	return nil, nil
}
func (r *fakeSkillRepo) SaveObservations(context.Context, string, []core.Observation) error {
	return nil
}
func (r *fakeSkillRepo) Observations(context.Context, int) ([]core.Observation, error) {
	return nil, nil
}
func (r *fakeSkillRepo) UpdateConversationSummary(context.Context, string, string) error {
	return nil
}
func (r *fakeSkillRepo) UpsertUserModel(context.Context, []core.UserModel) error { return nil }
func (r *fakeSkillRepo) RecordSkillUsage(context.Context, string) error          { return nil }

func (r *fakeSkillRepo) RecordSessionEvent(context.Context, string, string, string) error {
	return nil
}
func (r *fakeSkillRepo) Close() error { return nil }

func TestHub_LoadDirSyncsImportsAndIndexesAll(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeSkill(t, dir, "deploy.md", "---\nname: deploy-notes\ndescription: ship a release\ntrigger: deploy\n---\n\nsteps here\n")

	repo := &fakeSkillRepo{store: []core.Skill{
		{Name: "old-agent-skill", Description: "legacy agent skill", Source: core.SourceAgent},
	}}
	hub := NewHub(repo, discardLogger())

	if err := hub.LoadDir(ctx, dir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	saved := 0
	for _, s := range repo.store {
		if s.Name == "deploy-notes" && s.Source == core.SourceImported {
			saved++
		}
	}
	if saved != 1 {
		t.Errorf("imported skill persisted %d times, want exactly 1 (store = %+v)", saved, repo.store)
	}
	if len(hub.Skills()) != 2 {
		t.Fatalf("index has %d skills, want 2 (import + existing agent)", len(hub.Skills()))
	}
}

func TestHub_MatchANDSemantics(t *testing.T) {
	hub := NewHub(&fakeSkillRepo{}, discardLogger())
	hub.skills = []core.Skill{
		{Name: "deploy-notes", Trigger: "deploy", Description: "ship a release"},
		{Name: "release-checklist", Trigger: "release", Description: "pre-deploy checks"},
		{Name: "coffee-pref", Trigger: "coffee", Description: "user likes dark roast"},
	}

	tests := []struct {
		name  string
		input string
		limit int
		want  []string
	}{
		{"trigger term matches after stop words", "how do I deploy this", 0, []string{"deploy-notes", "release-checklist"}},
		{"case insensitive", "DEPLOY", 0, []string{"deploy-notes", "release-checklist"}},
		{"multi-term AND over fields", "deploy release notes", 0, []string{"deploy-notes"}},
		{"term outside haystack excludes", "coffee weather", 0, nil},
		{"shared term hits several, order kept", "deploy", 0, []string{"deploy-notes", "release-checklist"}},
		{"limit respected", "deploy", 1, []string{"deploy-notes"}},
		{"no match at all", "what is the weather", 0, nil},
		{"empty input", "   ", 0, nil},
	}

	for _, tt := range tests {
		got := hub.Match(tt.input, tt.limit)
		names := make([]string, len(got))
		for i, s := range got {
			names[i] = s.Name
		}
		if strings.Join(names, ",") != strings.Join(tt.want, ",") {
			t.Errorf("%s: Match(%q) = %v, want %v", tt.name, tt.input, names, tt.want)
		}
	}
}

// recordingUsageRepo wraps a repo to observe RecordSkillUsage reaching it.
type recordingUsageRepo struct {
	fakeSkillRepo
	usage []string
}

func (r *recordingUsageRepo) RecordSkillUsage(_ context.Context, name string) error {
	r.usage = append(r.usage, name)
	return nil
}

func TestHub_RecordUseReachesRepo(t *testing.T) {
	repo := &recordingUsageRepo{}
	hub := NewHub(repo, discardLogger())

	hub.RecordUse(context.Background(), "anything")

	if len(repo.usage) != 1 || repo.usage[0] != "anything" {
		t.Errorf("usage = %v, want one call for anything", repo.usage)
	}
}

func TestHub_AddUpsertsIndexEntry(t *testing.T) {
	hub := NewHub(&fakeSkillRepo{}, discardLogger())
	hub.Add(core.Skill{Name: "a", Content: "first"})
	hub.Add(core.Skill{Name: "a", Content: "second"})
	hub.Add(core.Skill{Name: "b"})

	if len(hub.Skills()) != 2 {
		t.Fatalf("index has %d skills, want 2", len(hub.Skills()))
	}
	for _, s := range hub.Skills() {
		if s.Name == "a" && s.Content != "second" {
			t.Errorf("Add did not replace by name: %+v", s)
		}
	}
}
