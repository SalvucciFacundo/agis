package core_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestAttachment_DomainModel(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic header
	att := core.Attachment{
		Type:     "image",
		MimeType: "image/png",
		Data:     data,
		URL:      "https://example.com/image.png",
		Name:     "screenshot.png",
	}

	if att.Type != "image" {
		t.Errorf("att.Type = %q, want %q", att.Type, "image")
	}
	if att.MimeType != "image/png" {
		t.Errorf("att.MimeType = %q, want %q", att.MimeType, "image/png")
	}
	if len(att.Data) != len(data) {
		t.Errorf("len(att.Data) = %d, want %d", len(att.Data), len(data))
	}
	if att.URL != "https://example.com/image.png" {
		t.Errorf("att.URL = %q, want %q", att.URL, "https://example.com/image.png")
	}
	if att.Name != "screenshot.png" {
		t.Errorf("att.Name = %q, want %q", att.Name, "screenshot.png")
	}
}

func TestMessage_WithAttachments_JSON(t *testing.T) {
	tests := []struct {
		name     string
		msg      core.Message
		wantJSON string
	}{
		{
			name: "text only message omits attachments in json",
			msg: core.Message{
				ID:             1,
				ConversationID: "conv-1",
				Role:           core.RoleUser,
				Content:        "hello",
				CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			wantJSON: `{"ID":1,"ConversationID":"conv-1","Role":"user","Content":"hello","CreatedAt":"2026-01-01T00:00:00Z","ToolCalls":null,"ToolCallID":""}`,
		},
		{
			name: "message with image and audio attachments",
			msg: core.Message{
				ID:             2,
				ConversationID: "conv-1",
				Role:           core.RoleUser,
				Content:        "look and listen",
				CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/png",
						Data:     []byte{1, 2, 3},
						Name:     "img.png",
					},
					{
						Type:     "audio",
						MimeType: "audio/ogg",
						Data:     []byte{4, 5, 6},
						Name:     "voice.ogg",
					},
				},
			},
			wantJSON: `{"ID":2,"ConversationID":"conv-1","Role":"user","Content":"look and listen","CreatedAt":"2026-01-01T00:00:00Z","ToolCalls":null,"ToolCallID":"","attachments":[{"Type":"image","MimeType":"image/png","Data":"AQID","URL":"","Name":"img.png"},{"Type":"audio","MimeType":"audio/ogg","Data":"BAUG","URL":"","Name":"voice.ogg"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			if string(b) != tt.wantJSON {
				t.Errorf("got json %s, want %s", string(b), tt.wantJSON)
			}

			var decoded core.Message
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if len(decoded.Attachments) != len(tt.msg.Attachments) {
				t.Errorf("decoded attachments len = %d, want %d", len(decoded.Attachments), len(tt.msg.Attachments))
			}
		})
	}
}
