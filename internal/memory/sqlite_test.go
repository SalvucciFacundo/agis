package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// openTestRepo returns a Repository over a fresh temp-file database, ready to
// use, with Close registered as cleanup.
func openTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(context.Background(), t.TempDir()+"/agis.db")
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestCreateAndLatestConversation(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "my session")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if conv.ID == "" {
		t.Error("conversation ID is empty")
	}
	if conv.Title != "my session" {
		t.Errorf("title = %q, want %q", conv.Title, "my session")
	}
	if conv.MessageCount != 0 {
		t.Errorf("message_count = %d, want 0", conv.MessageCount)
	}

	latest, err := repo.LatestConversation(ctx)
	if err != nil {
		t.Fatalf("LatestConversation() error = %v", err)
	}
	if latest.ID != conv.ID {
		t.Errorf("latest.ID = %q, want %q", latest.ID, conv.ID)
	}
}

func TestCreateConversation_EmptyTitleUsesDefault(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if conv.Title != defaultTitle {
		t.Errorf("title = %q, want default %q", conv.Title, defaultTitle)
	}
}

func TestLatestConversation_EmptyReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	_, err := repo.LatestConversation(ctx)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("LatestConversation() error = %v, want core.ErrNotFound", err)
	}
}

func TestAppendAndMessages_Order(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	contents := []string{"first", "second", "third"}
	for _, c := range contents {
		if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: c}); err != nil {
			t.Fatalf("AppendMessage(%q) error = %v", c, err)
		}
	}

	msgs, err := repo.Messages(ctx, conv.ID, 0)
	if err != nil {
		t.Fatalf("Messages() error = %v", err)
	}
	if len(msgs) != len(contents) {
		t.Fatalf("got %d messages, want %d", len(msgs), len(contents))
	}
	for i, c := range contents {
		if msgs[i].Content != c {
			t.Errorf("messages[%d].Content = %q, want %q", i, msgs[i].Content, c)
		}
		if msgs[i].Role != core.RoleUser {
			t.Errorf("messages[%d].Role = %q, want user", i, msgs[i].Role)
		}
		if msgs[i].ConversationID != conv.ID {
			t.Errorf("messages[%d].ConversationID = %q, want %q", i, msgs[i].ConversationID, conv.ID)
		}
	}
	// IDs must be strictly increasing (insertion order).
	for i := 1; i < len(msgs); i++ {
		if msgs[i].ID <= msgs[i-1].ID {
			t.Errorf("message IDs not increasing: %d then %d", msgs[i-1].ID, msgs[i].ID)
		}
	}
}

func TestMessages_TailLimit(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	for i := 0; i < 5; i++ {
		c := string(rune('a' + i))
		if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: c}); err != nil {
			t.Fatalf("AppendMessage(%q) error = %v", c, err)
		}
	}

	msgs, err := repo.Messages(ctx, conv.ID, 2)
	if err != nil {
		t.Fatalf("Messages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	// Tail of 5 = last 2, still in chronological order.
	if msgs[0].Content != "d" || msgs[1].Content != "e" {
		t.Errorf("tail = [%q %q], want [d e]", msgs[0].Content, msgs[1].Content)
	}
}

func TestAppendMessage_UpdatesCountAndTimestamp(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	before := conv.UpdatedAt

	for i := 0; i < 3; i++ {
		if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleAssistant, Content: "x"}); err != nil {
			t.Fatalf("AppendMessage() error = %v", err)
		}
	}

	latest, err := repo.LatestConversation(ctx)
	if err != nil {
		t.Fatalf("LatestConversation() error = %v", err)
	}
	if latest.MessageCount != 3 {
		t.Errorf("message_count = %d, want 3", latest.MessageCount)
	}
	if !latest.UpdatedAt.After(before) {
		t.Errorf("updated_at %v not after %v", latest.UpdatedAt, before)
	}
}

func TestLatestConversation_ReturnsMostRecentlyUpdated(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	first, err := repo.CreateConversation(ctx, "first")
	if err != nil {
		t.Fatalf("CreateConversation(first) error = %v", err)
	}
	second, err := repo.CreateConversation(ctx, "second")
	if err != nil {
		t.Fatalf("CreateConversation(second) error = %v", err)
	}

	// Bump the first conversation's updated_at so it becomes the latest.
	if err := repo.AppendMessage(ctx, first.ID, core.Message{Role: core.RoleUser, Content: "hi"}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	latest, err := repo.LatestConversation(ctx)
	if err != nil {
		t.Fatalf("LatestConversation() error = %v", err)
	}
	if latest.ID != first.ID {
		t.Errorf("latest.ID = %q, want %q (first)", latest.ID, first.ID)
	}
	if latest.ID == second.ID {
		t.Error("latest returned the untouched conversation")
	}
}

func TestDeleteConversation_Cascade(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	// Create conversation 1 with messages, attachments, and snapshots.
	conv1, err := repo.CreateConversation(ctx, "Session 1")
	if err != nil {
		t.Fatalf("CreateConversation(1) error = %v", err)
	}

	msg := core.Message{
		Role:    core.RoleUser,
		Content: "test message with attachment",
		Attachments: []core.Attachment{
			{
				Type:     "image",
				MimeType: "image/png",
				Data:     []byte{0x89, 0x50, 0x4E, 0x47},
				Name:     "photo.png",
			},
		},
	}
	if err := repo.AppendMessage(ctx, conv1.ID, msg); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	snap, err := repo.CreateSnapshot(ctx, conv1.ID)
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}
	if snap == nil {
		t.Fatal("expected snapshot, got nil")
	}

	// Create conversation 2 to verify isolation.
	conv2, err := repo.CreateConversation(ctx, "Session 2")
	if err != nil {
		t.Fatalf("CreateConversation(2) error = %v", err)
	}
	if err := repo.AppendMessage(ctx, conv2.ID, core.Message{Role: core.RoleUser, Content: "session 2 msg"}); err != nil {
		t.Fatalf("AppendMessage(2) error = %v", err)
	}

	// Delete conversation 1 via Repository method.
	if err := repo.DeleteConversation(ctx, conv1.ID); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	// Verify conversation 1 is gone.
	if _, err := repo.GetConversation(ctx, conv1.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetConversation(1) error = %v, want core.ErrNotFound", err)
	}

	// Verify messages for conversation 1 are cascaded.
	var msgCount int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE conversation_id = ?`, conv1.ID).Scan(&msgCount); err != nil {
		t.Fatalf("counting messages: %v", err)
	}
	if msgCount != 0 {
		t.Errorf("got %d messages after DeleteConversation, want 0", msgCount)
	}

	// Verify snapshots for conversation 1 are cascaded.
	snaps, err := repo.ListSnapshots(ctx, conv1.ID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("got %d snapshots after DeleteConversation, want 0", len(snaps))
	}

	// Verify attachments are cascaded.
	var attCount int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM attachments`).Scan(&attCount); err != nil {
		t.Fatalf("counting attachments: %v", err)
	}
	if attCount != 0 {
		t.Errorf("got %d attachments after DeleteConversation, want 0", attCount)
	}

	// Verify conversation 2 and its messages remain untouched.
	c2, err := repo.GetConversation(ctx, conv2.ID)
	if err != nil {
		t.Fatalf("GetConversation(2) error = %v", err)
	}
	if c2.ID != conv2.ID {
		t.Errorf("c2.ID = %q, want %q", c2.ID, conv2.ID)
	}
	msgs2, err := repo.Messages(ctx, conv2.ID, 0)
	if err != nil {
		t.Fatalf("Messages(2) error = %v", err)
	}
	if len(msgs2) != 1 {
		t.Errorf("got %d messages for conv2, want 1", len(msgs2))
	}
}

func TestDeleteConversation_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	err := repo.DeleteConversation(ctx, "non-existent-conv-id")
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("DeleteConversation(non-existent) error = %v, want core.ErrNotFound", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	conv, err := repo.CreateConversation(ctx, "")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := repo.AppendMessage(ctx, conv.ID, core.Message{Role: core.RoleUser, Content: "m"}); err != nil {
			t.Fatalf("AppendMessage() error = %v", err)
		}
	}

	// Deleting the conversation must cascade to its messages.
	if _, err := repo.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, conv.ID); err != nil {
		t.Fatalf("DELETE error = %v", err)
	}

	var n int
	if err := repo.db.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE conversation_id = ?`, conv.ID).Scan(&n); err != nil {
		t.Fatalf("counting messages: %v", err)
	}
	if n != 0 {
		t.Errorf("got %d messages after cascade delete, want 0", n)
	}
}

func TestAppendMessage_MissingConversationErrors(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

	err := repo.AppendMessage(ctx, "does-not-exist", core.Message{Role: core.RoleUser, Content: "hi"})
	if err == nil {
		t.Fatal("AppendMessage() error = nil, want foreign key violation")
	}
}
