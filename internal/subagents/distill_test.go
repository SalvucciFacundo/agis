package subagents

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func TestExtractKeyLearnings_HeadingsAndLists(t *testing.T) {
	tests := []struct {
		name     string
		task     string
		output   string
		maxObs   int
		convID   string
		wantObs  []core.Observation
	}{
		{
			name:   "extract numbered items from key learnings",
			task:   "Investigate auth cache",
			output: "Task completed.\n## Key Learnings\n1. The authentication service requires redis v7+.\n2. Cache keys must be prefixed with tenant ID.\n",
			maxObs: 3,
			convID: "conv-123",
			wantObs: []core.Observation{
				{
					TopicKey:   "subagent/investigate-auth-cache/1",
					Type:       "discovery",
					Content:    "The authentication service requires redis v7+.",
					Importance: 3,
					SourceRef:  "conv-123",
				},
				{
					TopicKey:   "subagent/investigate-auth-cache/2",
					Type:       "discovery",
					Content:    "Cache keys must be prefixed with tenant ID.",
					Importance: 3,
					SourceRef:  "conv-123",
				},
			},
		},
		{
			name:   "extract bullet items from decisions with colon",
			task:   "Tune database",
			output: "## Decisions:\n- Selected SQLite WAL mode for higher concurrency.\n* Configured 60s busy timeout.\n",
			maxObs: 3,
			convID: "conv-456",
			wantObs: []core.Observation{
				{
					TopicKey:   "subagent/tune-database/1",
					Type:       "decision",
					Content:    "Selected SQLite WAL mode for higher concurrency.",
					Importance: 3,
					SourceRef:  "conv-456",
				},
				{
					TopicKey:   "subagent/tune-database/2",
					Type:       "decision",
					Content:    "Configured 60s busy timeout.",
					Importance: 3,
					SourceRef:  "conv-456",
				},
			},
		},
		{
			name:   "case-insensitive headings Discoveries and Key Takeaways",
			task:   "Explore plugins",
			output: "## DISCOVERIES\n- Found plugin architecture in internal/plugins.\n\n## Key Takeaways:\n1. Plugins communicate via RPC.\n",
			maxObs: 3,
			convID: "conv-789",
			wantObs: []core.Observation{
				{
					TopicKey:   "subagent/explore-plugins/1",
					Type:       "discovery",
					Content:    "Found plugin architecture in internal/plugins.",
					Importance: 3,
					SourceRef:  "conv-789",
				},
				{
					TopicKey:   "subagent/explore-plugins/2",
					Type:       "discovery",
					Content:    "Plugins communicate via RPC.",
					Importance: 3,
					SourceRef:  "conv-789",
				},
			},
		},
		{
			name:    "output without structured learning sections returns empty slice",
			task:    "Regular task",
			output:  "This is regular prose without any special learning headers.\nJust answering the user query.",
			maxObs:  3,
			convID:  "conv-000",
			wantObs: []core.Observation{},
		},
		{
			name: "enforcing max observations ceiling",
			task: "Large audit",
			output: `## Key Learnings
1. Item 1
2. Item 2
3. Item 3
4. Item 4
5. Item 5
6. Item 6
`,
			maxObs: 3,
			convID: "conv-audit",
			wantObs: []core.Observation{
				{
					TopicKey:   "subagent/large-audit/1",
					Type:       "discovery",
					Content:    "Item 1",
					Importance: 3,
					SourceRef:  "conv-audit",
				},
				{
					TopicKey:   "subagent/large-audit/2",
					Type:       "discovery",
					Content:    "Item 2",
					Importance: 3,
					SourceRef:  "conv-audit",
				},
				{
					TopicKey:   "subagent/large-audit/3",
					Type:       "discovery",
					Content:    "Item 3",
					Importance: 3,
					SourceRef:  "conv-audit",
				},
			},
		},
		{
			name:   "strips whitespace and ignores blank list items",
			task:   "Clean text",
			output: "## Decisions\n-    \n- Valid decision item   \n* \n* Another decision\n",
			maxObs: 3,
			convID: "conv-clean",
			wantObs: []core.Observation{
				{
					TopicKey:   "subagent/clean-text/1",
					Type:       "decision",
					Content:    "Valid decision item",
					Importance: 3,
					SourceRef:  "conv-clean",
				},
				{
					TopicKey:   "subagent/clean-text/2",
					Type:       "decision",
					Content:    "Another decision",
					Importance: 3,
					SourceRef:  "conv-clean",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractKeyLearnings(tt.task, tt.output, tt.maxObs, tt.convID)
			if len(got) == 0 && len(tt.wantObs) == 0 {
				return
			}
			if len(got) != len(tt.wantObs) {
				t.Fatalf("ExtractKeyLearnings() len = %d, want %d (got: %+v)", len(got), len(tt.wantObs), got)
			}
			for i := range got {
				if got[i].TopicKey != tt.wantObs[i].TopicKey {
					t.Errorf("[%d] TopicKey = %q, want %q", i, got[i].TopicKey, tt.wantObs[i].TopicKey)
				}
				if got[i].Type != tt.wantObs[i].Type {
					t.Errorf("[%d] Type = %q, want %q", i, got[i].Type, tt.wantObs[i].Type)
				}
				if got[i].Content != tt.wantObs[i].Content {
					t.Errorf("[%d] Content = %q, want %q", i, got[i].Content, tt.wantObs[i].Content)
				}
				if got[i].Importance != tt.wantObs[i].Importance {
					t.Errorf("[%d] Importance = %d, want %d", i, got[i].Importance, tt.wantObs[i].Importance)
				}
				if got[i].SourceRef != tt.wantObs[i].SourceRef {
					t.Errorf("[%d] SourceRef = %q, want %q", i, got[i].SourceRef, tt.wantObs[i].SourceRef)
				}
			}
		})
	}
}

func TestSanitizeTaskSlug(t *testing.T) {
	tests := []struct {
		name string
		task string
		want string
	}{
		{
			name: "simple task",
			task: "Investigate auth cache",
			want: "investigate-auth-cache",
		},
		{
			name: "task with special characters and slashes",
			task: "Fix /internal/subagents/engine.go: Null pointer error!",
			want: "fix-internal-subagents-engine-go",
		},
		{
			name: "very long task truncated to 32 chars",
			task: "this is a very long task description that should definitely be truncated properly",
			want: "this-is-a-very-long-task-descrip",
		},
		{
			name: "empty or only special chars falls back to task",
			task: "??? --- !!!",
			want: "task",
		},
		{
			name: "consecutive spaces and hyphens collapsed",
			task: "foo   --   bar __ baz",
			want: "foo-bar-baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTaskSlug(tt.task)
			if got != tt.want {
				t.Errorf("sanitizeTaskSlug(%q) = %q, want %q", tt.task, got, tt.want)
			}
			if len(got) > 32 {
				t.Errorf("len(sanitizeTaskSlug) = %d > 32", len(got))
			}
		})
	}
}

func TestExtractKeyLearnings_MaxObservationsClamping(t *testing.T) {
	output := `## Key Learnings
1. One
2. Two
3. Three
4. Four
5. Five
6. Six
7. Seven
`
	// maxObs <= 0 should clamp to default 3
	gotZero := ExtractKeyLearnings("task", output, 0, "conv-1")
	if len(gotZero) != 3 {
		t.Errorf("len(gotZero) = %d, want 3", len(gotZero))
	}

	// maxObs > 5 should clamp to hard upper bound 5
	gotLarge := ExtractKeyLearnings("task", output, 10, "conv-1")
	if len(gotLarge) != 5 {
		t.Errorf("len(gotLarge) = %d, want 5", len(gotLarge))
	}
}
