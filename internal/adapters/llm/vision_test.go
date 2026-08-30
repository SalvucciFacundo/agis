package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestVision_MultipartPayload_BinaryDataURL(t *testing.T) {
	pngData := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDRfake")
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"I see a PNG image"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "What is in this image?",
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/png",
						Data:     pngData,
						Name:     "test.png",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}
	if resp.Content != "I see a PNG image" {
		t.Errorf("Chat() content = %q, want %q", resp.Content, "I see a PNG image")
	}

	var reqPayload struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(capturedBody, &reqPayload); err != nil {
		t.Fatalf("unmarshaling captured request: %v", err)
	}

	if len(reqPayload.Messages) != 1 {
		t.Fatalf("messages count = %d, want 1", len(reqPayload.Messages))
	}

	// Content must be an array of parts
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}
	if err := json.Unmarshal(reqPayload.Messages[0].Content, &parts); err != nil {
		t.Fatalf("unmarshaling content parts: %v (raw: %s)", err, string(reqPayload.Messages[0].Content))
	}

	if len(parts) != 2 {
		t.Fatalf("parts count = %d, want 2 (text + image_url)", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "What is in this image?" {
		t.Errorf("parts[0] = %+v, want text part", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
		t.Fatalf("parts[1] = %+v, want image_url part", parts[1])
	}

	expectedURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngData)
	if parts[1].ImageURL.URL != expectedURL {
		t.Errorf("image_url = %q, want %q", parts[1].ImageURL.URL, expectedURL)
	}
}

func TestVision_MultipartPayload_RemoteURL(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Image analyzed"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.Chat(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "Check remote image",
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/jpeg",
						URL:      "https://example.com/photo.jpg",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	var reqPayload struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(capturedBody, &reqPayload); err != nil {
		t.Fatalf("unmarshaling captured request: %v", err)
	}

	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}
	if err := json.Unmarshal(reqPayload.Messages[0].Content, &parts); err != nil {
		t.Fatalf("unmarshaling content parts: %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("parts count = %d, want 2", len(parts))
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/photo.jpg" {
		t.Errorf("image_url = %+v, want remote URL https://example.com/photo.jpg", parts[1].ImageURL)
	}
}

func TestVision_MIMEValidation(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	// Attachment with unsupported MIME and an audio attachment
	_, err := client.Chat(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "Ignore unsupported attachments",
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/bmp", // unsupported
						Data:     []byte("bmpdata"),
					},
					{
						Type:     "audio",
						MimeType: "audio/ogg", // non-image
						Data:     []byte("oggdata"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	var reqPayload struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(capturedBody, &reqPayload); err != nil {
		t.Fatalf("unmarshaling captured request: %v", err)
	}

	// Since no valid vision attachments remain, content should be serialized as standard string
	var strContent string
	if err := json.Unmarshal(reqPayload.Messages[0].Content, &strContent); err != nil {
		t.Fatalf("content should be raw string when no valid images present, got: %s", string(reqPayload.Messages[0].Content))
	}
	if strContent != "Ignore unsupported attachments" {
		t.Errorf("content = %q, want %q", strContent, "Ignore unsupported attachments")
	}
}

func TestVision_TextOnly_BackwardCompatibility(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello back"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.Chat(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "Pure text message",
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	var reqPayload struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(capturedBody, &reqPayload); err != nil {
		t.Fatalf("unmarshaling captured request: %v", err)
	}

	var strContent string
	if err := json.Unmarshal(reqPayload.Messages[0].Content, &strContent); err != nil {
		t.Fatalf("content should be string: %v", err)
	}
	if strContent != "Pure text message" {
		t.Errorf("content = %q, want %q", strContent, "Pure text message")
	}
}

func TestVision_MultipleImages(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"analyzed multiple"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.Chat(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "Compare these two",
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/gif",
						Data:     []byte("GIF89a..."),
					},
					{
						Type:     "image",
						MimeType: "image/jpeg",
						URL:      "https://example.com/b.jpg",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}

	var reqPayload struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(capturedBody, &reqPayload); err != nil {
		t.Fatalf("unmarshaling captured request: %v", err)
	}

	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text,omitempty"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}
	if err := json.Unmarshal(reqPayload.Messages[0].Content, &parts); err != nil {
		t.Fatalf("unmarshaling content parts: %v", err)
	}

	if len(parts) != 3 {
		t.Fatalf("parts count = %d, want 3 (text + 2 image_urls)", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "Compare these two" {
		t.Errorf("parts[0] = %+v, want text part", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL.URL != "data:image/gif;base64,"+base64.StdEncoding.EncodeToString([]byte("GIF89a...")) {
		t.Errorf("parts[1] = %+v, want gif data url", parts[1])
	}
	if parts[2].Type != "image_url" || parts[2].ImageURL.URL != "https://example.com/b.jpg" {
		t.Errorf("parts[2] = %+v, want remote url", parts[2])
	}
}

func TestVision_StreamWithImages(t *testing.T) {
	server := newSSEServer(t,
		`{"choices":[{"delta":{"content":"I see"}}]}`,
		`{"choices":[{"delta":{"content":" images"}}]}`,
		`[DONE]`,
	)
	client := NewClient(server.URL, "test-key")

	ch, err := client.Stream(context.Background(), core.ChatRequest{
		Model: "gpt-4o",
		Messages: []core.Message{
			{
				Role:    core.RoleUser,
				Content: "What do you see?",
				Attachments: []core.Attachment{
					{
						Type:     "image",
						MimeType: "image/png",
						Data:     []byte("pngdata"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v, want nil", err)
	}

	var got strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("Stream event err = %v, want nil", ev.Err)
		}
		got.WriteString(ev.Text)
	}
	if got.String() != "I see images" {
		t.Errorf("Stream text = %q, want %q", got.String(), "I see images")
	}
}

