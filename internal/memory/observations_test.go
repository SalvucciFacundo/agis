package memory

import (
	"context"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestSaveObservations_CreatesAndRecalls(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	obs := []core.Observation{
		{TopicKey: "user/prefs/coffee", Type: "preference", Content: "dark roast", Importance: 4},
		{TopicKey: "project/arch", Type: "note", Content: "hexagonal", Importance: 3},
	}
	if err := repo.SaveObservations(ctx, "conv-1", obs); err != nil {
		t.Fatalf("SaveObservations() error = %v", err)
	}

	got, err := repo.Observations(ctx, 0)
	if err != nil {
		t.Fatalf("Observations() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d observations, want 2", len(got))
	}
	for _, o := range got {
		if o.SourceRef != "conv-1" {
			t.Errorf("SourceRef = %q, want conv-1", o.SourceRef)
		}
		if o.ID == "" {
			t.Error("observation ID is empty")
		}
	}
}

func TestSaveObservations_UpsertPreservesCreatedAtBumpsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	if err := repo.SaveObservations(ctx, "conv-1", []core.Observation{
		{TopicKey: "user/prefs/coffee", Type: "preference", Content: "dark roast", Importance: 4},
	}); err != nil {
		t.Fatalf("SaveObservations(first) error = %v", err)
	}

	before, err := repo.Observations(ctx, 0)
	if err != nil {
		t.Fatalf("Observations() error = %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("got %d observations, want 1", len(before))
	}
	createdAt := before[0].CreatedAt

	time.Sleep(2 * time.Millisecond)

	if err := repo.SaveObservations(ctx, "conv-2", []core.Observation{
		{TopicKey: "user/prefs/coffee", Type: "preference", Content: "light roast", Importance: 5},
	}); err != nil {
		t.Fatalf("SaveObservations(second) error = %v", err)
	}

	after, err := repo.Observations(ctx, 0)
	if err != nil {
		t.Fatalf("Observations() error = %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("got %d observations after upsert, want 1", len(after))
	}
	if after[0].ID != before[0].ID {
		t.Errorf("id changed on upsert: %q -> %q", before[0].ID, after[0].ID)
	}
	if !after[0].CreatedAt.Equal(createdAt) {
		t.Errorf("created_at changed on upsert: %v -> %v", createdAt, after[0].CreatedAt)
	}
	if !after[0].UpdatedAt.After(createdAt) {
		t.Errorf("updated_at %v not after created_at %v", after[0].UpdatedAt, createdAt)
	}
	if after[0].Content != "light roast" {
		t.Errorf("content = %q, want %q", after[0].Content, "light roast")
	}
	if after[0].SourceRef != "conv-2" {
		t.Errorf("source_ref = %q, want conv-2 (latest producer)", after[0].SourceRef)
	}
}

func TestSaveObservations_ImportanceClamped(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	obs := []core.Observation{
		{TopicKey: "a", Type: "note", Content: "zero", Importance: 0},
		{TopicKey: "b", Type: "note", Content: "negative", Importance: -3},
		{TopicKey: "c", Type: "note", Content: "high", Importance: 10},
		{TopicKey: "d", Type: "note", Content: "ok", Importance: 4},
	}
	if err := repo.SaveObservations(ctx, "conv-1", obs); err != nil {
		t.Fatalf("SaveObservations() error = %v", err)
	}

	got, err := repo.Observations(ctx, 0)
	if err != nil {
		t.Fatalf("Observations() error = %v", err)
	}
	want := map[string]int{"a": 3, "b": 1, "c": 5, "d": 4}
	if len(got) != len(want) {
		t.Fatalf("got %d observations, want %d", len(got), len(want))
	}
	for _, o := range got {
		if want[o.TopicKey] != o.Importance {
			t.Errorf("importance for %q = %d, want %d", o.TopicKey, o.Importance, want[o.TopicKey])
		}
	}
}

func TestSaveObservations_BatchAtomic(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	obs := []core.Observation{
		{TopicKey: "good-1", Type: "note", Content: "first", Importance: 3},
		{TopicKey: "   ", Type: "note", Content: "bad", Importance: 3}, // empty topic_key
		{TopicKey: "good-2", Type: "note", Content: "third", Importance: 3},
	}
	if err := repo.SaveObservations(ctx, "conv-1", obs); err == nil {
		t.Fatal("SaveObservations() error = nil, want error for empty topic_key")
	}

	var n int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM observations`).Scan(&n); err != nil {
		t.Fatalf("counting observations: %v", err)
	}
	if n != 0 {
		t.Errorf("got %d observations, want 0 (batch must roll back atomically)", n)
	}

	var fts int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM memory_fts WHERE doc_type = ?`, docTypeObservation).Scan(&fts); err != nil {
		t.Fatalf("counting observation FTS rows: %v", err)
	}
	if fts != 0 {
		t.Errorf("got %d observation FTS rows, want 0", fts)
	}
}

func TestObservations_NewestFirst(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	if err := repo.SaveObservations(ctx, "conv-1", []core.Observation{
		{TopicKey: "older", Type: "note", Content: "old", Importance: 3},
	}); err != nil {
		t.Fatalf("SaveObservations(older) error = %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := repo.SaveObservations(ctx, "conv-1", []core.Observation{
		{TopicKey: "newer", Type: "note", Content: "new", Importance: 3},
	}); err != nil {
		t.Fatalf("SaveObservations(newer) error = %v", err)
	}

	got, err := repo.Observations(ctx, 0)
	if err != nil {
		t.Fatalf("Observations() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d observations, want 2", len(got))
	}
	if got[0].TopicKey != "newer" || got[1].TopicKey != "older" {
		t.Errorf("order = [%q %q], want [newer older]", got[0].TopicKey, got[1].TopicKey)
	}

	limited, err := repo.Observations(ctx, 1)
	if err != nil {
		t.Fatalf("Observations(1) error = %v", err)
	}
	if len(limited) != 1 || limited[0].TopicKey != "newer" {
		t.Errorf("Observations(1) = %+v, want [newer]", limited)
	}
}

func TestUpdateConversationSummary_DoesNotBumpUpdatedAt(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "session")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	before := conv.UpdatedAt

	time.Sleep(2 * time.Millisecond)

	if err := repo.UpdateConversationSummary(ctx, conv.ID, "summarized"); err != nil {
		t.Fatalf("UpdateConversationSummary() error = %v", err)
	}

	latest, err := repo.LatestConversation(ctx)
	if err != nil {
		t.Fatalf("LatestConversation() error = %v", err)
	}
	if latest.Summary != "summarized" {
		t.Errorf("summary = %q, want %q", latest.Summary, "summarized")
	}
	if !latest.UpdatedAt.Equal(before) {
		t.Errorf("updated_at changed on summary write: %v -> %v", before, latest.UpdatedAt)
	}
}

func TestRecordSessionEvent_PersistsAndValidates(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	if err := repo.RecordSessionEvent(ctx, "conv-1", "nudge", `{"count":10}`); err != nil {
		t.Fatalf("RecordSessionEvent(nudge) error = %v", err)
	}

	var kind, payload string
	if err := repo.db.QueryRowContext(ctx,
		`SELECT kind, payload FROM session_events WHERE session_id = ?`, "conv-1").Scan(&kind, &payload); err != nil {
		t.Fatalf("reading session event: %v", err)
	}
	if kind != "nudge" {
		t.Errorf("kind = %q, want nudge", kind)
	}
	if payload != `{"count":10}` {
		t.Errorf("payload = %q, want the recorded payload", payload)
	}

	// An unknown kind is rejected rather than violating the CHECK constraint
	// with an opaque error.
	if err := repo.RecordSessionEvent(ctx, "conv-1", "bogus", ""); err == nil {
		t.Fatal("RecordSessionEvent(bogus) error = nil, want error")
	}
}

func TestUpsertUserModel_Upserts(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	if err := repo.UpsertUserModel(ctx, []core.UserModel{
		{Key: "user/prefs/coffee", Value: "dark roast", Confidence: 0.8},
	}); err != nil {
		t.Fatalf("UpsertUserModel(first) error = %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	if err := repo.UpsertUserModel(ctx, []core.UserModel{
		{Key: "user/prefs/coffee", Value: "light roast", Confidence: 0.9},
	}); err != nil {
		t.Fatalf("UpsertUserModel(second) error = %v", err)
	}

	var n int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM user_model`).Scan(&n); err != nil {
		t.Fatalf("counting user model rows: %v", err)
	}
	if n != 1 {
		t.Fatalf("got %d user model rows, want 1", n)
	}

	var value string
	var confidence float64
	if err := repo.db.QueryRowContext(ctx,
		`SELECT value, confidence FROM user_model WHERE key = ?`, "user/prefs/coffee").Scan(&value, &confidence); err != nil {
		t.Fatalf("reading user model: %v", err)
	}
	if value != "light roast" {
		t.Errorf("value = %q, want %q", value, "light roast")
	}
	if confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", confidence)
	}
}
