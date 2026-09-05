package subagents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/google/uuid"
)

// ephemeralRepository implements core.Repository for isolated child subagent execution.
// It keeps conversation, message, and snapshot state in-memory to prevent pollution
// of the parent persistent storage, while proxying shared knowledge (search,
// observations, skills, user model, audit) to the parent repository.
type ephemeralRepository struct {
	parent    core.Repository
	convs     map[string]*core.Conversation
	latest    *core.Conversation
	messages  map[string][]core.Message
	snapshots map[string][]core.Snapshot
	mu        sync.RWMutex
}

// NewEphemeralRepository creates an isolated in-memory repository wrapping parent.
func NewEphemeralRepository(parent core.Repository) core.Repository {
	return &ephemeralRepository{
		parent:    parent,
		convs:     make(map[string]*core.Conversation),
		messages:  make(map[string][]core.Message),
		snapshots: make(map[string][]core.Snapshot),
	}
}

func (r *ephemeralRepository) CreateConversation(_ context.Context, title string) (*core.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("subagent-%s", uuid.NewString())
	conv := &core.Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.convs[id] = conv
	r.latest = conv
	return conv, nil
}

func (r *ephemeralRepository) LatestConversation(_ context.Context) (*core.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.latest == nil {
		return nil, core.ErrNotFound
	}
	c := *r.latest
	return &c, nil
}

func (r *ephemeralRepository) AppendMessage(_ context.Context, convID string, msg core.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.messages[convID] = append(r.messages[convID], msg)
	if c, ok := r.convs[convID]; ok {
		c.UpdatedAt = time.Now()
	}
	return nil
}

func (r *ephemeralRepository) Messages(_ context.Context, convID string, limit int) ([]core.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	msgs := r.messages[convID]
	if len(msgs) == 0 {
		return []core.Message{}, nil
	}
	if limit <= 0 || limit >= len(msgs) {
		copied := make([]core.Message, len(msgs))
		copy(copied, msgs)
		return copied, nil
	}

	start := len(msgs) - limit
	copied := make([]core.Message, limit)
	copy(copied, msgs[start:])
	return copied, nil
}

func (r *ephemeralRepository) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.Search(ctx, query, limit)
}

func (r *ephemeralRepository) SaveObservations(ctx context.Context, convID string, obs []core.Observation) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.SaveObservations(ctx, convID, obs)
}

func (r *ephemeralRepository) Observations(ctx context.Context, limit int) ([]core.Observation, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.Observations(ctx, limit)
}

func (r *ephemeralRepository) UpdateConversationSummary(ctx context.Context, convID, summary string) error {
	r.mu.Lock()
	if c, ok := r.convs[convID]; ok {
		c.Summary = summary
	}
	r.mu.Unlock()

	if r.parent != nil {
		return r.parent.UpdateConversationSummary(ctx, convID, summary)
	}
	return nil
}

func (r *ephemeralRepository) UpsertUserModel(ctx context.Context, rows []core.UserModel) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.UpsertUserModel(ctx, rows)
}

func (r *ephemeralRepository) UserModelRows(ctx context.Context, limit int) ([]core.UserModel, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.UserModelRows(ctx, limit)
}

func (r *ephemeralRepository) ClearUserModel(ctx context.Context) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.ClearUserModel(ctx)
}

func (r *ephemeralRepository) RecordSessionEvent(ctx context.Context, sessionID, kind, payload string) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.RecordSessionEvent(ctx, sessionID, kind, payload)
}

func (r *ephemeralRepository) SaveSkill(ctx context.Context, skill core.Skill) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.SaveSkill(ctx, skill)
}

func (r *ephemeralRepository) ListSkills(ctx context.Context) ([]core.Skill, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.ListSkills(ctx)
}

func (r *ephemeralRepository) RecordSkillUsage(ctx context.Context, name string) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.RecordSkillUsage(ctx, name)
}

func (r *ephemeralRepository) AppendAudit(ctx context.Context, entry core.AuditEntry) error {
	if r.parent == nil {
		return nil
	}
	return r.parent.AppendAudit(ctx, entry)
}

func (r *ephemeralRepository) AuditTail(ctx context.Context, n int) ([]core.AuditEntry, error) {
	if r.parent == nil {
		return nil, nil
	}
	return r.parent.AuditTail(ctx, n)
}

func (r *ephemeralRepository) ListConversations(_ context.Context, limit, offset int) ([]core.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]core.Conversation, 0, len(r.convs))
	for _, c := range r.convs {
		list = append(list, *c)
	}

	if offset >= len(list) {
		return []core.Conversation{}, nil
	}
	end := len(list)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return list[offset:end], nil
}

func (r *ephemeralRepository) GetConversation(_ context.Context, id string) (*core.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.convs[id]
	if !ok {
		return nil, core.ErrNotFound
	}
	copied := *c
	return &copied, nil
}

func (r *ephemeralRepository) RenameConversation(_ context.Context, id, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.convs[id]
	if !ok {
		return core.ErrNotFound
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	return nil
}

func (r *ephemeralRepository) DeleteConversation(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.convs, id)
	delete(r.messages, id)
	delete(r.snapshots, id)
	if r.latest != nil && r.latest.ID == id {
		r.latest = nil
	}
	return nil
}

func (r *ephemeralRepository) CreateSnapshot(_ context.Context, convID string) (*core.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	snap := &core.Snapshot{
		ID:             fmt.Sprintf("snap-%s", uuid.NewString()),
		ConversationID: convID,
		CreatedAt:      time.Now(),
	}
	r.snapshots[convID] = append(r.snapshots[convID], *snap)
	return snap, nil
}

func (r *ephemeralRepository) ListSnapshots(_ context.Context, convID string) ([]core.Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snaps := r.snapshots[convID]
	res := make([]core.Snapshot, len(snaps))
	copy(res, snaps)
	return res, nil
}

func (r *ephemeralRepository) Close() error {
	if r.parent != nil {
		return r.parent.Close()
	}
	return nil
}
