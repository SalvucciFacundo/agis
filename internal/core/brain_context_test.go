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
	skills  []Skill
	used    []string
}

func (f *fakeHub) Match(input string, limit int) []Skill {
	if len(f.matches) > limit {
		return f.matches[:limit]
	}
	return f.matches
}

func (f *fakeHub) Skills() []Skill {
	return f.skills
}

func (f *fakeHub) RecordUse(_ context.Context, name string) { f.used = append(f.used, name) }

func (f *fakeHub) GetSkill(name string) (*Skill, bool) {
	for _, s := range f.skills {
		if s.Name == name {
			return &s, true
		}
	}
	for _, s := range f.matches {
		if s.Name == name {
			return &s, true
		}
	}
	return nil, false
}

func (f *fakeHub) Reload(_ context.Context, _ string) error {
	return nil
}

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

func TestBrainStep_SkillsLazyLoadingPrompt(t *testing.T) {
	repo := newFakeRepo()
	hub := &fakeHub{matches: []Skill{
		{
			Name:        "deploy-notes",
			Description: "Procedures to ship",
			Trigger:     "deploy",
			Source:      "imported",
			Content:     "## Workflow\n1. tag then push",
		},
	}}
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(
		repo,
		provider,
		WithSkills(hub),
		WithSkillsLazyLoading(true),
	)

	if err := brain.Step(context.Background(), "help me deploy"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs := provider.requests[0].Messages
	var skillMsg string
	for _, m := range msgs {
		if m.Role == RoleSystem && strings.Contains(m.Content, "Applicable skills") {
			skillMsg = m.Content
			break
		}
	}
	if skillMsg == "" {
		t.Fatalf("no skills system message found in %+v", msgs)
	}
	if !strings.Contains(skillMsg, "| Skill | Trigger / Description | Scope / Version |") {
		t.Errorf("skillMsg = %q, want compact table header", skillMsg)
	}
	if !strings.Contains(skillMsg, "To read complete procedural instructions, call read_skill(name)") {
		t.Errorf("skillMsg = %q, want read_skill instructions", skillMsg)
	}
	if strings.Contains(skillMsg, "tag then push") {
		t.Errorf("skillMsg contains full body %q, want lazy index only", skillMsg)
	}
}

func TestBrainStep_SkillsFullBodyPrompt(t *testing.T) {
	repo := newFakeRepo()
	hub := &fakeHub{matches: []Skill{
		{
			Name:        "deploy-notes",
			Description: "Procedures to ship",
			Trigger:     "deploy",
			Source:      "imported",
			Content:     "## Workflow\n1. tag then push",
		},
	}}
	provider := &capturingProvider{events: []StreamEvent{{Text: "ok"}}}
	brain := NewBrain(
		repo,
		provider,
		WithSkills(hub),
		WithSkillsLazyLoading(false),
	)

	if err := brain.Step(context.Background(), "help me deploy"); err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	msgs := provider.requests[0].Messages
	var skillMsg string
	for _, m := range msgs {
		if m.Role == RoleSystem && strings.Contains(m.Content, "Applicable skills") {
			skillMsg = m.Content
			break
		}
	}
	if skillMsg == "" {
		t.Fatalf("no skills system message found in %+v", msgs)
	}
	if !strings.Contains(skillMsg, "- deploy-notes: ## Workflow\n1. tag then push") {
		t.Errorf("skillMsg = %q, want full body injection", skillMsg)
	}
	if strings.Contains(skillMsg, "call read_skill(name)") {
		t.Errorf("skillMsg = %q, should not contain lazy prompt when disabled", skillMsg)
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
