package skills

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// fakeChatProvider is a core.Provider double returning a fixed Chat response.
type fakeChatProvider struct {
	resp core.ChatResponse
	err  error

	requests []core.ChatRequest
}

func (f *fakeChatProvider) Chat(_ context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	f.requests = append(f.requests, req)
	return f.resp, f.err
}

func (f *fakeChatProvider) Stream(context.Context, core.ChatRequest) (<-chan core.StreamEvent, error) {
	return nil, errors.New("Stream not used by the creator")
}

func (f *fakeChatProvider) Models() []core.ModelInfo { return nil }

// creatorRepo wraps a stateful repo to observe skill persistence and session
// events.
type creatorRepo struct {
	fakeSkillRepo
	events []recordedEvent
}

type recordedEvent struct {
	sessionID string
	kind      string
	payload   string
}

func (r *creatorRepo) RecordSessionEvent(_ context.Context, sessionID, kind, payload string) error {
	r.events = append(r.events, recordedEvent{sessionID: sessionID, kind: kind, payload: payload})
	return nil
}

func TestCreator_ExtractsAndPersists(t *testing.T) {
	ctx := context.Background()
	provider := &fakeChatProvider{resp: core.ChatResponse{Content: `{"name":"release-steps","description":"ship it","trigger":"release","content":"1. check\n2. tag"}`}}
	repo := &creatorRepo{}
	creator := NewCreator(provider, repo, true, discardLogger())

	skill, err := creator.Extract(ctx, "conv-1", []core.Message{{Role: core.RoleUser, Content: "we shipped like this..."}})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if skill == nil || skill.Name != "release-steps" || skill.Source != core.SourceAgent {
		t.Fatalf("skill = %+v, want an agent-sourced skill", skill)
	}
	if len(repo.store) != 1 || repo.store[0].Name != "release-steps" {
		t.Errorf("store = %+v, want the saved skill", repo.store)
	}
	if len(repo.events) != 1 || repo.events[0].kind != "skill" || !strings.Contains(repo.events[0].payload, "release-steps") {
		t.Errorf("events = %+v, want one skill event naming the skill", repo.events)
	}

	// The provider saw the system prompt first and the conversation after.
	req := provider.requests[0]
	if len(req.Messages) != 2 || req.Messages[0].Role != core.RoleSystem {
		t.Errorf("messages = %+v, want system prompt + conversation", req.Messages)
	}
}

func TestCreator_NullResponseMeansNothing(t *testing.T) {
	provider := &fakeChatProvider{resp: core.ChatResponse{Content: "null"}}
	repo := &creatorRepo{}
	creator := NewCreator(provider, repo, true, discardLogger())

	skill, err := creator.Extract(context.Background(), "conv-1", nil)
	if err != nil || skill != nil {
		t.Errorf("Extract() = %v, %v; want nil, nil for null response", skill, err)
	}
}

func TestCreator_MalformedSkipsWithoutPersisting(t *testing.T) {
	for _, content := range []string{"not json at all", `{"name":"","description":"d","content":"c"}`, ``} {
		provider := &fakeChatProvider{resp: core.ChatResponse{Content: content}}
		repo := &creatorRepo{}
		creator := NewCreator(provider, repo, true, discardLogger())

		skill, err := creator.Extract(context.Background(), "conv-1", nil)
		if err != nil || skill != nil {
			t.Errorf("content %q: Extract() = %v, %v; want skip", content, skill, err)
		}
		if len(repo.store) != 0 || len(repo.events) != 0 {
			t.Errorf("content %q: persisted writes happened: %+v", content, repo)
		}
	}
}

func TestCreator_FencedJSONParses(t *testing.T) {
	provider := &fakeChatProvider{resp: core.ChatResponse{Content: "```json\n{\"name\":\"fenced\",\"description\":\"d\",\"content\":\"c\"}\n```"}}
	repo := &creatorRepo{}
	creator := NewCreator(provider, repo, true, discardLogger())

	skill, err := creator.Extract(context.Background(), "conv-1", nil)
	if err != nil || skill == nil || skill.Name != "fenced" {
		t.Errorf("Extract() = %v, %v; want fenced JSON parsed", skill, err)
	}
}

func TestCreator_ChatErrorReturns(t *testing.T) {
	provider := &fakeChatProvider{err: errors.New("provider down")}
	repo := &creatorRepo{}
	creator := NewCreator(provider, repo, true, discardLogger())

	if _, err := creator.Extract(context.Background(), "conv-1", nil); err == nil {
		t.Fatal("Extract() error = nil, want infrastructure error")
	}
	if len(repo.store) != 0 {
		t.Error("persistence happened despite chat failure")
	}
}

func TestCreator_DisabledIsNoop(t *testing.T) {
	provider := &fakeChatProvider{resp: core.ChatResponse{Content: `{"name":"x","description":"y","content":"z"}`}}
	repo := &creatorRepo{}
	creator := NewCreator(provider, repo, false, discardLogger())

	skill, err := creator.Extract(context.Background(), "conv-1", nil)
	if err != nil || skill != nil {
		t.Errorf("Extract() = %v, %v; want disabled no-op", skill, err)
	}
	if len(provider.requests) != 0 {
		t.Errorf("Chat called %d times while disabled, want 0", len(provider.requests))
	}
}
