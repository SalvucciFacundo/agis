package memory

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestParseObservations(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []core.Observation
		wantErr bool
	}{
		{
			name:    "valid array",
			content: `[{"topic_key":"user/prefs/coffee","type":"preference","content":"dark roast","importance":4}]`,
			want:    []core.Observation{{TopicKey: "user/prefs/coffee", Type: "preference", Content: "dark roast", Importance: 4}},
		},
		{
			name:    "fenced json",
			content: "```json\n[{\"topic_key\":\"a\",\"type\":\"note\",\"content\":\"x\",\"importance\":2}]\n```",
			want:    []core.Observation{{TopicKey: "a", Type: "note", Content: "x", Importance: 2}},
		},
		{
			name:    "missing importance defaults to 3",
			content: `[{"topic_key":"a","type":"note","content":"x"}]`,
			want:    []core.Observation{{TopicKey: "a", Type: "note", Content: "x", Importance: 3}},
		},
		{
			name:    "empty array",
			content: `[]`,
			want:    []core.Observation{},
		},
		{
			name:    "prose is malformed",
			content: "Sure, here are some observations about you.",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseObservations(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseObservations() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseObservations() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d observations, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].TopicKey != tt.want[i].TopicKey ||
					got[i].Type != tt.want[i].Type ||
					got[i].Content != tt.want[i].Content ||
					got[i].Importance != tt.want[i].Importance {
					t.Errorf("obs[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStripFences(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"bare array", `[{"a":1}]`, `[{"a":1}]`},
		{"json fence", "```json\n[1]\n```", "[1]"},
		{"bare fence", "```\n[1]\n```", "[1]"},
		{"leading whitespace and fence", "  ```json\n[1]\n```  ", "[1]"},
		{"no trailing newline", "```\n[1]```", "[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripFences(tt.content); got != tt.want {
				t.Errorf("stripFences(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestCurator_Nudge_PersistsObservations(t *testing.T) {
	provider := &fakeChatProvider{chatResp: core.ChatResponse{
		Content: `[{"topic_key":"user/prefs/coffee","type":"preference","content":"dark roast","importance":4}]`,
	}}
	repo := &recordingRepo{}
	curator := NewCurator(provider, repo, slog.New(slog.DiscardHandler))

	obs, err := curator.Nudge(context.Background(), "conv-1", []core.Message{{Role: core.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Nudge() error = %v", err)
	}
	if len(obs) != 1 || obs[0].TopicKey != "user/prefs/coffee" || obs[0].Importance != 4 {
		t.Errorf("Nudge() obs = %+v, want the parsed observation", obs)
	}
	if len(repo.savedObservations) != 1 {
		t.Fatalf("SaveObservations got %d rows, want 1", len(repo.savedObservations))
	}
	if repo.savedObservations[0].Content != "dark roast" {
		t.Errorf("saved content = %q, want %q", repo.savedObservations[0].Content, "dark roast")
	}

	if len(provider.requests) != 1 {
		t.Fatalf("Chat called %d times, want 1", len(provider.requests))
	}
	req := provider.requests[0]
	if len(req.Messages) == 0 || req.Messages[0].Role != core.RoleSystem {
		t.Errorf("first message = %+v, want the system curator prompt", req.Messages)
	}
	if len(req.Messages) != 2 || req.Messages[1].Content != "hi" {
		t.Errorf("conversation messages not carried into the request: %+v", req.Messages)
	}
}

func TestCurator_Nudge_MalformedSkips(t *testing.T) {
	provider := &fakeChatProvider{chatResp: core.ChatResponse{Content: "I have no idea what JSON is."}}
	repo := &recordingRepo{}
	curator := NewCurator(provider, repo, slog.New(slog.DiscardHandler))

	obs, err := curator.Nudge(context.Background(), "conv-1", nil)
	if err != nil {
		t.Fatalf("Nudge() error = %v, want nil (parse failure logs and skips)", err)
	}
	if obs != nil {
		t.Errorf("Nudge() obs = %v, want nil", obs)
	}
	if len(repo.savedObservations) != 0 {
		t.Errorf("SaveObservations called with %d rows, want 0", len(repo.savedObservations))
	}
}

func TestCurator_Nudge_ChatErrorReturns(t *testing.T) {
	provider := &fakeChatProvider{chatErr: errors.New("provider down")}
	repo := &recordingRepo{}
	curator := NewCurator(provider, repo, slog.New(slog.DiscardHandler))

	if _, err := curator.Nudge(context.Background(), "conv-1", nil); err == nil {
		t.Fatal("Nudge() error = nil, want provider error")
	}
	if len(repo.savedObservations) != 0 {
		t.Errorf("SaveObservations called on provider error")
	}
}
