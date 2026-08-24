package memory

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestSummarizer_Close_WritesSummaryObsAndModel(t *testing.T) {
	provider := &fakeChatProvider{chatResp: core.ChatResponse{Content: `{
		"summary": "discussed coffee and architecture",
		"observations": [
			{"topic_key":"user/pref/coffee","type":"preference","content":"dark roast","importance":4},
			{"topic_key":"project/arch","type":"note","content":"hexagonal","importance":3}
		]
	}`}}
	repo := &recordingRepo{}
	summarizer := NewSummarizer(provider, repo, slog.New(slog.DiscardHandler))

	if err := summarizer.Close(context.Background(), "conv-1", []core.Message{{Role: core.RoleUser, Content: "hi"}}); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Write order: summary → observations → user_model.
	idxSummary := indexOf(repo.calls, "UpdateConversationSummary")
	idxObs := indexOf(repo.calls, "SaveObservations")
	idxModel := indexOf(repo.calls, "UpsertUserModel")
	if idxSummary < 0 || idxObs < 0 || idxModel < 0 {
		t.Fatalf("missing writes: calls = %v", repo.calls)
	}
	if !(idxSummary < idxObs && idxObs < idxModel) {
		t.Errorf("write order wrong: calls = %v", repo.calls)
	}

	if repo.summary != "discussed coffee and architecture" {
		t.Errorf("summary = %q, want the parsed summary", repo.summary)
	}
	if repo.summaryConvID != "conv-1" {
		t.Errorf("summary convID = %q, want conv-1", repo.summaryConvID)
	}
	if len(repo.savedObservations) != 2 {
		t.Fatalf("got %d saved observations, want 2", len(repo.savedObservations))
	}

	// Only the user/* observation aggregates into the user model.
	if len(repo.savedUserModel) != 1 {
		t.Fatalf("got %d user model rows, want 1", len(repo.savedUserModel))
	}
	if repo.savedUserModel[0].Key != "user/pref/coffee" {
		t.Errorf("user model key = %q, want %q", repo.savedUserModel[0].Key, "user/pref/coffee")
	}
	if repo.savedUserModel[0].Confidence != 0.8 {
		t.Errorf("confidence = %v, want 0.8", repo.savedUserModel[0].Confidence)
	}
}

func TestSummarizer_Close_MalformedSkips(t *testing.T) {
	provider := &fakeChatProvider{chatResp: core.ChatResponse{Content: "not json"}}
	repo := &recordingRepo{}
	summarizer := NewSummarizer(provider, repo, slog.New(slog.DiscardHandler))

	if err := summarizer.Close(context.Background(), "conv-1", nil); err != nil {
		t.Fatalf("Close() error = %v, want nil (malformed logs and skips)", err)
	}
	if len(repo.calls) != 0 {
		t.Errorf("repo calls = %v, want none", repo.calls)
	}
}

func TestSummarizer_Close_ChatErrorReturns(t *testing.T) {
	provider := &fakeChatProvider{chatErr: errors.New("provider down")}
	repo := &recordingRepo{}
	summarizer := NewSummarizer(provider, repo, slog.New(slog.DiscardHandler))

	if err := summarizer.Close(context.Background(), "conv-1", nil); err == nil {
		t.Fatal("Close() error = nil, want provider error")
	}
	if len(repo.calls) != 0 {
		t.Errorf("repo calls = %v, want none on provider error", repo.calls)
	}
}

// indexOf returns the index of the first occurrence of want in calls, or -1.
func indexOf(calls []string, want string) int {
	for i, c := range calls {
		if c == want {
			return i
		}
	}
	return -1
}
