package memory

import (
	"context"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestSearch_AccentInsensitive(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "configuración"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	// Unaccented query must match the accented persisted content.
	results, err := repo.Search(ctx, "configuracion", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Content != "configuración" {
		t.Errorf("content = %q, want %q", results[0].Content, "configuración")
	}
	if results[0].DocType != docTypeMessage {
		t.Errorf("doc_type = %q, want %q", results[0].DocType, docTypeMessage)
	}
}

func TestSearch_ReturnsBothDocTypes(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "hola"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	// Observations are schema-only in M1; insert an FTS row directly to prove
	// Search spans both doc types rather than only messages.
	if _, err := repo.db.ExecContext(ctx,
		`INSERT INTO memory_fts (doc_type, doc_id, content) VALUES (?, ?, ?)`,
		docTypeObservation, "obs-1", "hola mundo"); err != nil {
		t.Fatalf("inserting observation FTS row: %v", err)
	}

	results, err := repo.Search(ctx, "hola", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	var seenMessage, seenObservation bool
	for _, r := range results {
		switch r.DocType {
		case docTypeMessage:
			seenMessage = true
		case docTypeObservation:
			seenObservation = true
			if r.DocID != "obs-1" {
				t.Errorf("observation doc_id = %q, want %q", r.DocID, "obs-1")
			}
		}
	}
	if !seenMessage {
		t.Error("Search() returned no message doc_type")
	}
	if !seenObservation {
		t.Error("Search() returned no observation doc_type")
	}
}

func TestSearch_EmptyQueryReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	results, err := repo.Search(ctx, "   ", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestSearch_Limit(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "needle"}); err != nil {
			t.Fatalf("AppendMessage() error = %v", err)
		}
	}

	results, err := repo.Search(ctx, "needle", 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

// TestAppendMessage_FailureRollsBackFTS proves the FTS row is written in the
// same transaction as the message: a failed append (foreign key violation on a
// missing conversation) must not leave an orphan search row behind.
func TestAppendMessage_FailureRollsBackFTS(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	err := repo.AppendMessage(ctx, "missing-conv", core.Message{Role: core.RoleUser, Content: "orphan-probe"})
	if err == nil {
		t.Fatal("AppendMessage() error = nil, want foreign key violation")
	}

	results, err := repo.Search(ctx, "orphan-probe", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (FTS row must roll back with the failed append)", len(results))
	}
}

// TestSearch_ImmediatelyVisibleAfterAppend proves the FTS sync is synchronous:
// once AppendMessage returns, the content is searchable with no extra step.
func TestSearch_ImmediatelyVisibleAfterAppend(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "sync-check"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	results, err := repo.Search(ctx, "sync-check", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (searchable immediately after append)", len(results))
	}
}
