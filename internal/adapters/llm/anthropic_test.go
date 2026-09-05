package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/adapters/llm"
	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestAnthropic_Chat(t *testing.T) {
	var capturedReq struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		System    string `json:"system"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q, want /v1/messages", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		_ = json.NewDecoder(r.Body).Decode(&capturedReq)

		resp := map[string]any{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "Hello from Claude!",
				},
			},
			"stop_reason": "end_turn",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := config.LLMConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet-20241022",
		APIKey:   "sk-ant-test-key",
		BaseURL:  srv.URL,
	}

	adapter := llm.NewAnthropic(cfg)

	req := core.ChatRequest{
		Messages: []core.Message{
			{Role: core.RoleSystem, Content: "You are a helpful assistant."},
			{Role: core.RoleUser, Content: "Hi!"},
		},
	}

	res, err := adapter.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if res.Content != "Hello from Claude!" {
		t.Errorf("res.Content = %q, want 'Hello from Claude!'", res.Content)
	}

	if capturedHeaders.Get("x-api-key") != "sk-ant-test-key" {
		t.Errorf("x-api-key = %q, want 'sk-ant-test-key'", capturedHeaders.Get("x-api-key"))
	}
	if capturedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want '2023-06-01'", capturedHeaders.Get("anthropic-version"))
	}
	if capturedReq.System != "You are a helpful assistant." {
		t.Errorf("system = %q, want 'You are a helpful assistant.'", capturedReq.System)
	}
	if len(capturedReq.Messages) != 1 || capturedReq.Messages[0].Content != "Hi!" {
		t.Errorf("messages = %+v, want 1 user message 'Hi!'", capturedReq.Messages)
	}
}

func TestAnthropic_Chat_ToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":   "msg_tool_1",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "Checking weather...",
				},
				{
					"type":  "tool_use",
					"id":    "toolu_01A",
					"name":  "get_weather",
					"input": map[string]any{"location": "San Francisco, CA"},
				},
			},
			"stop_reason": "tool_use",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := config.LLMConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet-20241022",
		APIKey:   "sk-ant-test",
		BaseURL:  srv.URL,
	}
	adapter := llm.NewAnthropic(cfg)

	req := core.ChatRequest{
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What is the weather in SF?"},
		},
		Tools: []core.ToolDef{
			{
				Name:        "get_weather",
				Description: "Get current weather",
			},
		},
	}

	res, err := adapter.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if res.Content != "Checking weather..." {
		t.Errorf("Content = %q, want 'Checking weather...'", res.Content)
	}
}

func TestAnthropic_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		events := []string{
			`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant"}}` + "\n\n",
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n",
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello "}}` + "\n\n",
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world!"}}` + "\n\n",
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n",
			`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}` + "\n\n",
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n",
		}

		for _, ev := range events {
			_, _ = fmt.Fprint(w, ev)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	cfg := config.LLMConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet-20241022",
		APIKey:   "sk-ant-test",
		BaseURL:  srv.URL,
	}
	adapter := llm.NewAnthropic(cfg)

	req := core.ChatRequest{
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Hello"},
		},
	}

	stream, err := adapter.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var text string
	for ev := range stream {
		if ev.Err != nil {
			t.Fatalf("unexpected stream error: %v", ev.Err)
		}
		text += ev.Text
	}

	if text != "Hello world!" {
		t.Errorf("streamed text = %q, want 'Hello world!'", text)
	}
}

func TestAnthropic_Stream_ToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		events := []string{
			`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant"}}` + "\n\n",
			`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_99","name":"search","input":{}}}` + "\n\n",
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}` + "\n\n",
			`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"golang\"}"}}` + "\n\n",
			`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n",
			`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n",
		}

		for _, ev := range events {
			_, _ = fmt.Fprint(w, ev)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	cfg := config.LLMConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet-20241022",
		APIKey:   "sk-ant-test",
		BaseURL:  srv.URL,
	}
	adapter := llm.NewAnthropic(cfg)

	req := core.ChatRequest{
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "Search golang"},
		},
	}

	stream, err := adapter.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var toolCalls []*core.ToolCall
	for ev := range stream {
		if ev.Err != nil {
			t.Fatalf("unexpected stream error: %v", ev.Err)
		}
		if ev.ToolCall != nil {
			toolCalls = append(toolCalls, ev.ToolCall)
		}
	}

	if len(toolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(toolCalls))
	}
	if toolCalls[0].ID != "toolu_99" || toolCalls[0].Name != "search" || toolCalls[0].Arguments != `{"q":"golang"}` {
		t.Errorf("toolCall = %+v, want toolu_99 / search / {\"q\":\"golang\"}", toolCalls[0])
	}
}

func TestAnthropic_Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		})
	}))
	defer srv.Close()

	cfg := config.LLMConfig{
		Provider: "anthropic",
		Model:    "claude-3-5-sonnet-20241022",
		APIKey:   "sk-ant-invalid",
		BaseURL:  srv.URL,
	}
	adapter := llm.NewAnthropic(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := adapter.Chat(ctx, core.ChatRequest{Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}}})
	if err == nil {
		t.Fatal("expected error on 401 response, got nil")
	}
}
