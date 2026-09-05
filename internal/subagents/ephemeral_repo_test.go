package subagents_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/subagents"
)

// fakeParentRepo tracks parent repository calls to verify isolation and proxying.
type fakeParentRepo struct {
	mu            sync.Mutex
	convs         map[string]*core.Conversation
	messages      map[string][]core.Message
	observations  []core.Observation
	userModels    []core.UserModel
	skills        map[string]core.Skill
	auditEntries  []core.AuditEntry
	sessionEvents []string
	closed        bool
}

func newFakeParentRepo() *fakeParentRepo {
	return &fakeParentRepo{
		convs:        make(map[string]*core.Conversation),
		messages:     make(map[string][]core.Message),
		observations: make([]core.Observation, 0),
		userModels:   make([]core.UserModel, 0),
		skills:       make(map[string]core.Skill),
		auditEntries: make([]core.AuditEntry, 0),
	}
}

func (r *fakeParentRepo) CreateConversation(_ context.Context, title string) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conv := &core.Conversation{ID: "parent-conv-1", Title: title, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.convs[conv.ID] = conv
	return conv, nil
}

func (r *fakeParentRepo) LatestConversation(_ context.Context) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.convs) == 0 {
		return nil, core.ErrNotFound
	}
	return r.convs["parent-conv-1"], nil
}

func (r *fakeParentRepo) AppendMessage(_ context.Context, convID string, msg core.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[convID] = append(r.messages[convID], msg)
	return nil
}

func (r *fakeParentRepo) Messages(_ context.Context, convID string, _ int) ([]core.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.messages[convID], nil
}

func (r *fakeParentRepo) Search(_ context.Context, query string, _ int) ([]core.SearchResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return []core.SearchResult{{DocType: "obs", DocID: "test", Content: "found " + query}}, nil
}

func (r *fakeParentRepo) SaveObservations(_ context.Context, _ string, obs []core.Observation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observations = append(r.observations, obs...)
	return nil
}

func (r *fakeParentRepo) Observations(_ context.Context, _ int) ([]core.Observation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.observations, nil
}

func (r *fakeParentRepo) UpdateConversationSummary(_ context.Context, _, _ string) error {
	return nil
}

func (r *fakeParentRepo) UpsertUserModel(_ context.Context, rows []core.UserModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userModels = append(r.userModels, rows...)
	return nil
}

func (r *fakeParentRepo) UserModelRows(_ context.Context, _ int) ([]core.UserModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userModels, nil
}

func (r *fakeParentRepo) ClearUserModel(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userModels = nil
	return nil
}

func (r *fakeParentRepo) RecordSessionEvent(_ context.Context, _, kind, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionEvents = append(r.sessionEvents, kind)
	return nil
}

func (r *fakeParentRepo) SaveSkill(_ context.Context, skill core.Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name] = skill
	return nil
}

func (r *fakeParentRepo) ListSkills(_ context.Context) ([]core.Skill, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]core.Skill, 0, len(r.skills))
	for _, s := range r.skills {
		res = append(res, s)
	}
	return res, nil
}

func (r *fakeParentRepo) RecordSkillUsage(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.skills[name]
	if !ok {
		return core.ErrNotFound
	}
	s.UsageCount++
	r.skills[name] = s
	return nil
}

func (r *fakeParentRepo) AppendAudit(_ context.Context, entry core.AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditEntries = append(r.auditEntries, entry)
	return nil
}

func (r *fakeParentRepo) AuditTail(_ context.Context, _ int) ([]core.AuditEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.auditEntries, nil
}

func (r *fakeParentRepo) ListConversations(_ context.Context, _, _ int) ([]core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]core.Conversation, 0, len(r.convs))
	for _, c := range r.convs {
		res = append(res, *c)
	}
	return res, nil
}

func (r *fakeParentRepo) GetConversation(_ context.Context, id string) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return nil, core.ErrNotFound
	}
	return c, nil
}

func (r *fakeParentRepo) RenameConversation(_ context.Context, id, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return core.ErrNotFound
	}
	c.Title = title
	return nil
}

func (r *fakeParentRepo) DeleteConversation(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.convs, id)
	delete(r.messages, id)
	return nil
}

func (r *fakeParentRepo) CreateSnapshot(_ context.Context, convID string) (*core.Snapshot, error) {
	return &core.Snapshot{ID: "snap-1", ConversationID: convID}, nil
}

func (r *fakeParentRepo) ListSnapshots(_ context.Context, _ string) ([]core.Snapshot, error) {
	return nil, nil
}

func (r *fakeParentRepo) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func TestEphemeralRepo_ConversationAndMessageIsolation(t *testing.T) {
	ctx := context.Background()
	parent := newFakeParentRepo()
	repo := subagents.NewEphemeralRepository(parent)

	// 1. Initial latest conversation is ErrNotFound
	_, err := repo.LatestConversation(ctx)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected ErrNotFound before creation, got: %v", err)
	}

	// 2. Create conversation in ephemeral repo
	conv, err := repo.CreateConversation(ctx, "subagent task")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if conv == nil || conv.ID == "" {
		t.Fatalf("expected valid conversation, got: %+v", conv)
	}

	// Ensure parent repo has NOT received this conversation
	parent.mu.Lock()
	if len(parent.convs) != 0 {
		t.Errorf("parent repo should have 0 conversations, got %d", len(parent.convs))
	}
	parent.mu.Unlock()

	// 3. LatestConversation returns the created conversation
	latest, err := repo.LatestConversation(ctx)
	if err != nil {
		t.Fatalf("LatestConversation failed: %v", err)
	}
	if latest.ID != conv.ID {
		t.Errorf("expected LatestConversation ID %q, got %q", conv.ID, latest.ID)
	}

	// 4. Append messages to ephemeral repo
	msg1 := core.Message{Role: core.RoleUser, Content: "do task"}
	msg2 := core.Message{Role: core.RoleAssistant, Content: "task done"}
	if err := repo.AppendMessage(ctx, conv.ID, msg1); err != nil {
		t.Fatalf("AppendMessage 1 failed: %v", err)
	}
	if err := repo.AppendMessage(ctx, conv.ID, msg2); err != nil {
		t.Fatalf("AppendMessage 2 failed: %v", err)
	}

	// Verify messages in ephemeral repo
	msgs, err := repo.Messages(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("Messages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "do task" || msgs[1].Content != "task done" {
		t.Errorf("unexpected message contents: %+v", msgs)
	}

	// Ensure parent repo has NOT received any messages
	parent.mu.Lock()
	if len(parent.messages) != 0 {
		t.Errorf("parent repo messages should be empty, got %d", len(parent.messages))
	}
	parent.mu.Unlock()

	// 5. GetConversation
	gotConv, err := repo.GetConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if gotConv.ID != conv.ID {
		t.Errorf("GetConversation ID = %q, want %q", gotConv.ID, conv.ID)
	}

	// Unknown conv ID returns ErrNotFound
	_, err = repo.GetConversation(ctx, "non-existent")
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown ID, got %v", err)
	}

	// 6. ListConversations
	list, err := repo.ListConversations(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != conv.ID {
		t.Errorf("ListConversations = %+v, want 1 item with ID %q", list, conv.ID)
	}

	// 7. RenameConversation
	if err := repo.RenameConversation(ctx, conv.ID, "updated title"); err != nil {
		t.Fatalf("RenameConversation failed: %v", err)
	}
	renamed, _ := repo.GetConversation(ctx, conv.ID)
	if renamed.Title != "updated title" {
		t.Errorf("renamed title = %q, want 'updated title'", renamed.Title)
	}

	// 8. DeleteConversation
	if err := repo.DeleteConversation(ctx, conv.ID); err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}
	_, err = repo.GetConversation(ctx, conv.ID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestEphemeralRepo_DelegatesSharedStateToParent(t *testing.T) {
	ctx := context.Background()
	parent := newFakeParentRepo()
	repo := subagents.NewEphemeralRepository(parent)

	// 1. Search
	results, err := repo.Search(ctx, "query", 5)
	if err != nil || len(results) != 1 || results[0].Content != "found query" {
		t.Errorf("Search failed or didn't delegate: %v, %+v", err, results)
	}

	// 2. Observations
	obs := []core.Observation{{TopicKey: "obs1", Content: "val1"}}
	if err := repo.SaveObservations(ctx, "conv1", obs); err != nil {
		t.Fatalf("SaveObservations failed: %v", err)
	}
	savedObs, err := repo.Observations(ctx, 5)
	if err != nil || len(savedObs) != 1 || savedObs[0].TopicKey != "obs1" {
		t.Errorf("Observations failed or didn't delegate: %v, %+v", err, savedObs)
	}

	// 3. User models
	if err := repo.UpsertUserModel(ctx, []core.UserModel{{Key: "k1", Value: "v1"}}); err != nil {
		t.Fatalf("UpsertUserModel failed: %v", err)
	}
	rows, err := repo.UserModelRows(ctx, 10)
	if err != nil || len(rows) != 1 || rows[0].Key != "k1" {
		t.Errorf("UserModelRows failed: %v, %+v", err, rows)
	}
	if err := repo.ClearUserModel(ctx); err != nil {
		t.Fatalf("ClearUserModel failed: %v", err)
	}
	rows, _ = repo.UserModelRows(ctx, 10)
	if len(rows) != 0 {
		t.Errorf("expected 0 user model rows after clear, got %d", len(rows))
	}

	// 4. Skills
	skill := core.Skill{Name: "skill1", Description: "desc"}
	if err := repo.SaveSkill(ctx, skill); err != nil {
		t.Fatalf("SaveSkill failed: %v", err)
	}
	skills, err := repo.ListSkills(ctx)
	if err != nil || len(skills) != 1 || skills[0].Name != "skill1" {
		t.Errorf("ListSkills failed: %v, %+v", err, skills)
	}
	if err := repo.RecordSkillUsage(ctx, "skill1"); err != nil {
		t.Fatalf("RecordSkillUsage failed: %v", err)
	}

	// 5. Audit
	audit := core.AuditEntry{Backend: "subagent", Category: "execution", Subject: "task"}
	if err := repo.AppendAudit(ctx, audit); err != nil {
		t.Fatalf("AppendAudit failed: %v", err)
	}
	entries, err := repo.AuditTail(ctx, 5)
	if err != nil || len(entries) != 1 || entries[0].Subject != "task" {
		t.Errorf("AuditTail failed: %v, %+v", err, entries)
	}

	// 6. Session event
	if err := repo.RecordSessionEvent(ctx, "conv1", "nudge", "payload"); err != nil {
		t.Fatalf("RecordSessionEvent failed: %v", err)
	}
	if len(parent.sessionEvents) != 1 || parent.sessionEvents[0] != "nudge" {
		t.Errorf("RecordSessionEvent not delegated properly")
	}

	// 7. Close
	if err := repo.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !parent.closed {
		t.Errorf("parent repo not marked closed")
	}
}

func TestEphemeralRepo_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	parent := newFakeParentRepo()
	repo := subagents.NewEphemeralRepository(parent)

	conv, err := repo.CreateConversation(ctx, "concurrent test")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "msg"})
			_, _ = repo.Messages(ctx, conv.ID, 100)
			_, _ = repo.GetConversation(ctx, conv.ID)
			_, _ = repo.LatestConversation(ctx)
		}(i)
	}
	wg.Wait()

	msgs, err := repo.Messages(ctx, conv.ID, 100)
	if err != nil {
		t.Fatalf("Messages error: %v", err)
	}
	if len(msgs) != 20 {
		t.Errorf("expected 20 messages, got %d", len(msgs))
	}
}
