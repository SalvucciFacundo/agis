package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestSaveSkill_InsertAndUpsert(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	skill := core.Skill{
		Name:        "deploy-notes",
		Description: "How to ship a release",
		Trigger:     "deploy",
		Content:     "1. run checks\n2. tag\n3. push",
		Source:      core.SourceImported,
	}
	if err := repo.SaveSkill(ctx, skill); err != nil {
		t.Fatalf("SaveSkill() error = %v", err)
	}

	got, err := repo.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d skills, want 1", len(got))
	}
	if got[0].Name != "deploy-notes" || got[0].Source != core.SourceImported {
		t.Errorf("skill = %+v, want the inserted row", got[0])
	}
	if got[0].UsageCount != 0 || !got[0].LastUsed.IsZero() {
		t.Errorf("fresh skill = %+v, want zero usage", got[0])
	}
	id := got[0].ID
	createdAt := got[0].CreatedAt

	// Re-saving the same name updates content but preserves identity and
	// usage state.
	updated := skill
	updated.Content = "1. run checks\n2. tag -m v\n3. push --tags"
	updated.Source = core.SourceAgent
	if err := repo.SaveSkill(ctx, updated); err != nil {
		t.Fatalf("SaveSkill(upsert) error = %v", err)
	}

	got, err = repo.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d skills after upsert, want 1", len(got))
	}
	if got[0].ID != id {
		t.Errorf("id changed after upsert: %q -> %q", id, got[0].ID)
	}
	if !got[0].CreatedAt.Equal(createdAt) {
		t.Errorf("created_at changed after upsert: %v -> %v", createdAt, got[0].CreatedAt)
	}
	if got[0].Content != updated.Content || got[0].Source != core.SourceAgent {
		t.Errorf("upsert did not refresh content/source: %+v", got[0])
	}
}

func TestRecordSkillUsage_BumpsTwice(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	if err := repo.SaveSkill(ctx, core.Skill{Name: "k", Content: "c", Source: core.SourceAgent}); err != nil {
		t.Fatalf("SaveSkill() error = %v", err)
	}

	before := time.Now().UTC().Add(-time.Second)
	for i := 0; i < 2; i++ {
		if err := repo.RecordSkillUsage(ctx, "k"); err != nil {
			t.Fatalf("RecordSkillUsage(%d) error = %v", i, err)
		}
	}

	got, err := repo.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d skills, want 1", len(got))
	}
	if got[0].UsageCount != 2 {
		t.Errorf("usage_count = %d, want 2", got[0].UsageCount)
	}
	if got[0].LastUsed.IsZero() || got[0].LastUsed.Before(before) {
		t.Errorf("last_used = %v, want stamped after %v", got[0].LastUsed, before)
	}
}

func TestListSkills_OrdersByLastUsedDescThenName(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	for _, s := range []core.Skill{
		{Name: "alpha", Content: "x", Source: core.SourceImported},
		{Name: "beta", Content: "x", Source: core.SourceImported},
		{Name: "gamma", Content: "x", Source: core.SourceImported},
	} {
		if err := repo.SaveSkill(ctx, s); err != nil {
			t.Fatalf("SaveSkill(%s) error = %v", s.Name, err)
		}
		time.Sleep(2 * time.Millisecond) // distinct last_used stamps
	}

	// Use gamma first, then beta: DESC order puts the most recently used
	// first, so expected order is beta, gamma, never-used alpha.
	if err := repo.RecordSkillUsage(ctx, "gamma"); err != nil {
		t.Fatalf("RecordSkillUsage(gamma) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.RecordSkillUsage(ctx, "beta"); err != nil {
		t.Fatalf("RecordSkillUsage(beta) error = %v", err)
	}

	got, err := repo.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	want := []string{"beta", "gamma", "alpha"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("position %d = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestSaveSkill_RejectsInvalid(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	tests := []struct {
		name  string
		skill core.Skill
	}{
		{"empty name", core.Skill{Name: "  ", Content: "c", Source: core.SourceImported}},
		{"empty content", core.Skill{Name: "n", Content: "", Source: core.SourceImported}},
		{"bad source", core.Skill{Name: "n", Content: "c", Source: "hacker"}},
	}
	for _, tt := range tests {
		if err := repo.SaveSkill(ctx, tt.skill); err == nil {
			t.Errorf("%s: SaveSkill() error = nil, want error", tt.name)
		}
	}
}

func TestRecordSkillUsage_UnknownNameNotFound(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	err := repo.RecordSkillUsage(ctx, "missing")
	if err == nil {
		t.Fatal("RecordSkillUsage(unknown) error = nil, want ErrNotFound")
	}
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("error = %v, want wrap of ErrNotFound", err)
	}
}
