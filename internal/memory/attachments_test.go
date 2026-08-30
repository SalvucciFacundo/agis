package memory

import (
	"bytes"
	"context"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"go.uber.org/goleak"
)

func newTestRepo(t *testing.T) (*Repository, context.Context) {
	t.Helper()
	ctx := context.Background()
	repo, err := NewRepository(ctx, t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	return repo, ctx
}

func TestRepository_AppendAndRetrieveAttachments(t *testing.T) {
	defer goleak.VerifyNone(t)
	repo, ctx := newTestRepo(t)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Multimodal Test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	rawImage := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0xFF, 0xFE, 0x00}
	rawAudio := []byte{0x4F, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0xAA, 0xBB, 0xCC}

	msg := core.Message{
		Role:    core.RoleUser,
		Content: "Analyze these media files",
		Attachments: []core.Attachment{
			{
				Type:     "image",
				MimeType: "image/png",
				Data:     rawImage,
				Name:     "chart.png",
				URL:      "https://cdn.example.com/chart.png",
			},
			{
				Type:     "audio",
				MimeType: "audio/ogg",
				Data:     rawAudio,
				Name:     "voice.ogg",
				URL:      "",
			},
		},
	}

	if err := repo.AppendMessage(ctx, conv.ID, msg); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	// Retrieve messages
	msgs, err := repo.Messages(ctx, conv.ID, 0)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}

	got := msgs[0]
	if got.Content != msg.Content {
		t.Errorf("got content %q, want %q", got.Content, msg.Content)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("got %d attachments, want 2", len(got.Attachments))
	}

	// Verify image attachment
	att1 := got.Attachments[0]
	if att1.Type != "image" {
		t.Errorf("att1.Type = %q, want %q", att1.Type, "image")
	}
	if att1.MimeType != "image/png" {
		t.Errorf("att1.MimeType = %q, want %q", att1.MimeType, "image/png")
	}
	if !bytes.Equal(att1.Data, rawImage) {
		t.Errorf("att1.Data binary mismatch")
	}
	if att1.Name != "chart.png" {
		t.Errorf("att1.Name = %q, want %q", att1.Name, "chart.png")
	}
	if att1.URL != "https://cdn.example.com/chart.png" {
		t.Errorf("att1.URL = %q, want %q", att1.URL, "https://cdn.example.com/chart.png")
	}

	// Verify audio attachment
	att2 := got.Attachments[1]
	if att2.Type != "audio" {
		t.Errorf("att2.Type = %q, want %q", att2.Type, "audio")
	}
	if att2.MimeType != "audio/ogg" {
		t.Errorf("att2.MimeType = %q, want %q", att2.MimeType, "audio/ogg")
	}
	if !bytes.Equal(att2.Data, rawAudio) {
		t.Errorf("att2.Data binary mismatch")
	}
	if att2.Name != "voice.ogg" {
		t.Errorf("att2.Name = %q, want %q", att2.Name, "voice.ogg")
	}
}

func TestRepository_TextOnlyMessageHasNoAttachments(t *testing.T) {
	defer goleak.VerifyNone(t)
	repo, ctx := newTestRepo(t)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Text Only Test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	msg := core.Message{
		Role:    core.RoleUser,
		Content: "Plain text turn",
	}

	if err := repo.AppendMessage(ctx, conv.ID, msg); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}

	msgs, err := repo.Messages(ctx, conv.ID, 0)
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if len(msgs[0].Attachments) != 0 {
		t.Errorf("got %d attachments for text-only message, want 0", len(msgs[0].Attachments))
	}
}

func TestRepository_CascadeDeleteConversationDeletesAttachments(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("applyMigrations: %v", err)
	}

	// Insert conversation, message, attachment
	_, err := db.ExecContext(ctx,
		`INSERT INTO conversations (id, title, created_at, updated_at, summary, message_count)
		 VALUES ('conv-del', 'To Delete', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '', 1)`)
	if err != nil {
		t.Fatalf("insert conv: %v", err)
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO messages (conversation_id, role, content, created_at)
		 VALUES ('conv-del', 'user', 'With media', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert msg: %v", err)
	}
	msgID, _ := res.LastInsertId()
	_, err = db.ExecContext(ctx,
		`INSERT INTO attachments (id, message_id, type, mime_type, data, url, name, created_at)
		 VALUES ('att-del', ?, 'image', 'image/png', X'89504E47', '', 'test.png', '2026-01-01T00:00:00Z')`, msgID)
	if err != nil {
		t.Fatalf("insert att: %v", err)
	}

	// Delete the conversation
	if _, err := db.ExecContext(ctx, `DELETE FROM conversations WHERE id = 'conv-del'`); err != nil {
		t.Fatalf("delete conv: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE id = 'att-del'`).Scan(&count); err != nil {
		t.Fatalf("query att: %v", err)
	}
	if count != 0 {
		t.Errorf("attachments count after conversation delete = %d, want 0", count)
	}
}

func TestRepository_MultipleMessagesWithMixedAttachmentsAndLimit(t *testing.T) {
	defer goleak.VerifyNone(t)
	repo, ctx := newTestRepo(t)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Mixed Messages")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	// Message 1: No attachments
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{
		Role:    core.RoleUser,
		Content: "Turn 1: text only",
	}); err != nil {
		t.Fatalf("AppendMessage 1: %v", err)
	}

	// Message 2: 1 image attachment
	imgData := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 1024) // 4KB
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{
		Role:    core.RoleUser,
		Content: "Turn 2: with image",
		Attachments: []core.Attachment{
			{
				Type:     "image",
				MimeType: "image/webp",
				Data:     imgData,
				Name:     "photo.webp",
			},
		},
	}); err != nil {
		t.Fatalf("AppendMessage 2: %v", err)
	}

	// Message 3: 2 audio attachments
	audio1 := []byte{1, 2, 3, 4}
	audio2 := []byte{5, 6, 7, 8}
	if err := repo.AppendMessage(ctx, conv.ID, core.Message{
		Role:    core.RoleAssistant,
		Content: "Turn 3: audio response",
		Attachments: []core.Attachment{
			{
				Type:     "audio",
				MimeType: "audio/wav",
				Data:     audio1,
				Name:     "part1.wav",
			},
			{
				Type:     "audio",
				MimeType: "audio/wav",
				Data:     audio2,
				Name:     "part2.wav",
			},
		},
	}); err != nil {
		t.Fatalf("AppendMessage 3: %v", err)
	}

	// Test retrieving all messages (limit = 0)
	allMsgs, err := repo.Messages(ctx, conv.ID, 0)
	if err != nil {
		t.Fatalf("Messages(0): %v", err)
	}
	if len(allMsgs) != 3 {
		t.Fatalf("len(allMsgs) = %d, want 3", len(allMsgs))
	}
	if len(allMsgs[0].Attachments) != 0 {
		t.Errorf("msg 0 attachments = %d, want 0", len(allMsgs[0].Attachments))
	}
	if len(allMsgs[1].Attachments) != 1 {
		t.Errorf("msg 1 attachments = %d, want 1", len(allMsgs[1].Attachments))
	} else {
		if !bytes.Equal(allMsgs[1].Attachments[0].Data, imgData) {
			t.Errorf("msg 1 attachment data mismatch")
		}
	}
	if len(allMsgs[2].Attachments) != 2 {
		t.Errorf("msg 2 attachments = %d, want 2", len(allMsgs[2].Attachments))
	} else {
		if !bytes.Equal(allMsgs[2].Attachments[0].Data, audio1) || !bytes.Equal(allMsgs[2].Attachments[1].Data, audio2) {
			t.Errorf("msg 2 attachment data mismatch")
		}
	}

	// Test retrieving tail with limit = 2 (messages 2 and 3)
	tailMsgs, err := repo.Messages(ctx, conv.ID, 2)
	if err != nil {
		t.Fatalf("Messages(2): %v", err)
	}
	if len(tailMsgs) != 2 {
		t.Fatalf("len(tailMsgs) = %d, want 2", len(tailMsgs))
	}
	if tailMsgs[0].Content != "Turn 2: with image" {
		t.Errorf("tailMsgs[0] content = %q, want %q", tailMsgs[0].Content, "Turn 2: with image")
	}
	if len(tailMsgs[0].Attachments) != 1 {
		t.Errorf("tailMsgs[0] attachments = %d, want 1", len(tailMsgs[0].Attachments))
	}
	if tailMsgs[1].Content != "Turn 3: audio response" {
		t.Errorf("tailMsgs[1] content = %q, want %q", tailMsgs[1].Content, "Turn 3: audio response")
	}
	if len(tailMsgs[1].Attachments) != 2 {
		t.Errorf("tailMsgs[1] attachments = %d, want 2", len(tailMsgs[1].Attachments))
	}
}
