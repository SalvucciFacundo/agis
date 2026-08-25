package core

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

// fakeHub is a SkillHub double returning fixed matches and recording usage.
type fakeHub struct {
	matches []Skill
	used    []string
}

func (f *fakeHub) Match(input string, limit int) []Skill {
	if len(f.matches) > limit {
		return f.matches[:limit]
	}
	return f.matches
}

func (f *fakeHub) RecordUse(_ context.Context, name string) { f.used = append(f.used, name) }

// fakeEvolution is an EvolutionLayer double with configurable text.
type fakeEvolution struct{ text string }

func (f fakeEvolution) Layer(context.Context) string { return f.text }

// fakeCreator is a SkillCreator double recording invocations.
type fakeCreator struct {
	calls int
	err   error
	skill *Skill
}

func (f *fakeCreator) Extract(context.Context, string, []Message) (*Skill, error) {
	f.calls++
	return f.skill, f.err
}

func TestBrainStep_ContextSlotOrder(t *testing.T) {
	repo := newFakeRepo()
	repo.observations = []Observation{{TopicKey: "user/pref", Content: "dark roast"}}
	hub := &fakeHub{matches: []Skill{{Name: "deploy-notes", Content: "tag then push"}}}
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(
		repo,
		provider,
		WithIdentity("You are AGIS."),
		WithSkills(hub),
		WithEvolution(fakeEvolution{text: "How to work with this user:\n- pref/coffee: dark roast"}),
	)

	if err := brain.Step(context.Background(), "help me deploy"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs := provider.requests[0].Messages
	if len(msgs) < 4 {
		t.Fatalf("got %d messages, want at least identity+skills+recall+tail", len(msgs))
	}
	for i := 0; i < 3; i++ {
		if msgs[i].Role != RoleSystem {
			t.Errorf("slot %d role = %v, want system", i, msgs[i].Role)
		}
	}
	if !strings.Contains(msgs[0].Content, "You are AGIS") ||
		!strings.Contains(msgs[0].Content, "pref/coffee") {
		t.Errorf("identity slot = %q, want SOUL + evolution composed", msgs[0].Content)
	}
	if !strings.Contains(msgs[1].Content, "deploy-notes") {
		t.Errorf("skills slot = %q, want the matched skill", msgs[1].Content)
	}
	if !strings.Contains(msgs[2].Content, "Relevant memories:") {
		t.Errorf("recall slot = %q, want the recall header", msgs[2].Content)
	}
	if msgs[len(msgs)-1].Role != RoleUser {
		t.Errorf("last message = %+v, want the user tail last", msgs[len(msgs)-1])
	}
	if len(hub.used) != 1 || hub.used[0] != "deploy-notes" {
		t.Errorf("RecordUse = %v, want one use of deploy-notes", hub.used)
	}
}

func TestBrainStep_BareMinimumSlots(t *testing.T) {
	repo := newFakeRepo()
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider) // no identity, no hub, no observations

	if err := brain.Step(context.Background(), "hello"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs := provider.requests[0].Messages
	if len(msgs) != 1 || msgs[0].Role != RoleUser {
		t.Errorf("messages = %+v, want just the user message with all slots empty", msgs)
	}
}

func TestBrain_OverlayJoinsIdentityNextTurn(t *testing.T) {
	repo := newFakeRepo()
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(repo, provider, WithIdentity("You are AGIS."))

	brain.SetOverlay("Be brief.")
	if err := brain.Step(context.Background(), "hi"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	first := provider.requests[0].Messages[0]
	if !strings.Contains(first.Content, "You are AGIS") || !strings.Contains(first.Content, "Be brief.") {
		t.Errorf("identity slot = %q, want SOUL + overlay composed", first.Content)
	}
}

func TestCloseSession_RunsCreatorAfterCloser(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{}
	creator := &fakeCreator{}
	brain := NewBrain(repo, &fakeProvider{}, WithSessionCloser(closer), WithSkillCreator(creator))

	if _, err := repo.CreateConversation(context.Background(), ""); err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if closer.calls != 1 {
		t.Errorf("closer calls = %d, want 1", closer.calls)
	}
	if creator.calls != 1 {
		t.Errorf("creator calls = %d, want 1 after the summarizer ran", creator.calls)
	}
}

func TestCloseSession_CreatorErrorNonFatal(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{}
	creator := &fakeCreator{err: context.DeadlineExceeded}
	brain := NewBrain(
		repo,
		&fakeProvider{},
		WithSessionCloser(closer),
		WithSkillCreator(creator),
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	if _, err := repo.CreateConversation(context.Background(), ""); err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	if err := brain.CloseSession(context.Background()); err != nil {
		t.Errorf("CloseSession() error = %v, want nil (extraction failure non-fatal)", err)
	}
	if creator.calls != 1 {
		t.Errorf("creator calls = %d, want 1", creator.calls)
	}
}

func TestCloseSession_NilCreatorSkips(t *testing.T) {
	repo := newFakeRepo()
	closer := &fakeCloser{}
	brain := NewBrain(repo, &fakeProvider{}, WithSessionCloser(closer))

	if _, err := repo.CreateConversation(context.Background(), ""); err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := brain.CloseSession(context.Background()); err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
}
