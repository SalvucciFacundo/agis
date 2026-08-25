package memory

import (
	"context"
	"errors"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// fakeChatProvider is a core.Provider double whose Chat returns a fixed
// response or error and records every request.
type fakeChatProvider struct {
	chatResp core.ChatResponse
	chatErr  error
	requests []core.ChatRequest
}

var _ core.Provider = (*fakeChatProvider)(nil)

func (f *fakeChatProvider) Chat(_ context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.chatErr != nil {
		return core.ChatResponse{}, f.chatErr
	}
	return f.chatResp, nil
}

func (f *fakeChatProvider) Stream(context.Context, core.ChatRequest) (<-chan core.StreamEvent, error) {
	return nil, errors.New("Stream not used")
}

func (f *fakeChatProvider) Models() []core.ModelInfo { return nil }

// recordingRepo is a core.Repository double that records the sequence of
// calls so tests can assert the learning-loop write order, plus the data each
// write received.
type recordingRepo struct {
	calls []string

	savedObservations []core.Observation
	savedUserModel    []core.UserModel
	summary           string
	summaryConvID     string
	events            []recordedEvent
	savedSkills       []core.Skill
	skills            []core.Skill
	usedSkill         string
	userModelRows     []core.UserModel
	auditEntries      []core.AuditEntry
}

type recordedEvent struct {
	sessionID string
	kind      string
	payload   string
}

var _ core.Repository = (*recordingRepo)(nil)

func (r *recordingRepo) CreateConversation(context.Context, string) (*core.Conversation, error) {
	r.calls = append(r.calls, "CreateConversation")
	return &core.Conversation{ID: "conv-1"}, nil
}

func (r *recordingRepo) LatestConversation(context.Context) (*core.Conversation, error) {
	r.calls = append(r.calls, "LatestConversation")
	return &core.Conversation{ID: "conv-1"}, nil
}

func (r *recordingRepo) AppendMessage(context.Context, string, core.Message) error {
	r.calls = append(r.calls, "AppendMessage")
	return nil
}

func (r *recordingRepo) Messages(context.Context, string, int) ([]core.Message, error) {
	r.calls = append(r.calls, "Messages")
	return nil, nil
}

func (r *recordingRepo) Search(context.Context, string, int) ([]core.SearchResult, error) {
	r.calls = append(r.calls, "Search")
	return nil, nil
}

func (r *recordingRepo) SaveObservations(_ context.Context, _ string, obs []core.Observation) error {
	r.calls = append(r.calls, "SaveObservations")
	r.savedObservations = append(r.savedObservations, obs...)
	return nil
}

func (r *recordingRepo) Observations(context.Context, int) ([]core.Observation, error) {
	r.calls = append(r.calls, "Observations")
	return nil, nil
}

func (r *recordingRepo) UpdateConversationSummary(_ context.Context, convID, summary string) error {
	r.calls = append(r.calls, "UpdateConversationSummary")
	r.summaryConvID = convID
	r.summary = summary
	return nil
}

func (r *recordingRepo) UpsertUserModel(_ context.Context, rows []core.UserModel) error {
	r.calls = append(r.calls, "UpsertUserModel")
	r.savedUserModel = append(r.savedUserModel, rows...)
	return nil
}

func (r *recordingRepo) RecordSessionEvent(_ context.Context, sessionID, kind, payload string) error {
	r.calls = append(r.calls, "RecordSessionEvent")
	r.events = append(r.events, recordedEvent{sessionID: sessionID, kind: kind, payload: payload})
	return nil
}

func (r *recordingRepo) SaveSkill(_ context.Context, skill core.Skill) error {
	r.calls = append(r.calls, "SaveSkill")
	r.savedSkills = append(r.savedSkills, skill)
	return nil
}

func (r *recordingRepo) ListSkills(context.Context) ([]core.Skill, error) {
	r.calls = append(r.calls, "ListSkills")
	return r.skills, nil
}

func (r *recordingRepo) RecordSkillUsage(_ context.Context, name string) error {
	r.calls = append(r.calls, "RecordSkillUsage")
	r.usedSkill = name
	return nil
}

func (r *recordingRepo) UserModelRows(context.Context, int) ([]core.UserModel, error) {
	r.calls = append(r.calls, "UserModelRows")
	return r.userModelRows, nil
}

func (r *recordingRepo) ClearUserModel(context.Context) error {
	r.calls = append(r.calls, "ClearUserModel")
	return nil
}

func (r *recordingRepo) AppendAudit(_ context.Context, e core.AuditEntry) error {
	r.calls = append(r.calls, "AppendAudit")
	r.auditEntries = append(r.auditEntries, e)
	return nil
}

func (r *recordingRepo) AuditTail(context.Context, int) ([]core.AuditEntry, error) {
	r.calls = append(r.calls, "AuditTail")
	return r.auditEntries, nil
}

func (r *recordingRepo) Close() error { return nil }
