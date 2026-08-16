package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// summarizerSystemPrompt instructs the provider to summarize the conversation
// and extract any final observations in one structured response.
const summarizerSystemPrompt = `You are a session summarizer. Summarize the conversation and extract any final durable observations about the user.

Respond with a single JSON object with exactly these fields:
- "summary": a concise summary of the session.
- "observations": a JSON array of facts worth remembering, each with "topic_key", "type", "content", and "importance" (1-5). User facts use the "user/" topic_key prefix. May be [].

Do not wrap the JSON in code fences.`

// Summarizer closes a session with a single Chat call returning {summary,
// observations[]}, then persists the summary, saves the observations, and
// aggregates the user model — all on the memory side so core never imports
// this package.
type Summarizer struct {
	provider core.Provider
	repo     core.Repository
	logger   *slog.Logger
}

var _ core.SessionCloser = (*Summarizer)(nil)

// NewSummarizer returns a Summarizer backed by provider and repo. A nil logger
// falls back to slog.Default().
func NewSummarizer(provider core.Provider, repo core.Repository, logger *slog.Logger) *Summarizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Summarizer{provider: provider, repo: repo, logger: logger}
}

// Close runs ONE Chat call returning {summary, observations[]}, then writes the
// summary (without bumping the conversation's updated_at), saves the
// observations, and aggregates the user model. Parse failures log and skip
// (returning nil); infrastructure errors are returned for the caller to treat
// as non-fatal.
func (s *Summarizer) Close(ctx context.Context, convID string, msgs []core.Message) error {
	resp, err := s.provider.Chat(ctx, core.ChatRequest{Messages: summarizeMessages(msgs)})
	if err != nil {
		return fmt.Errorf("summarizer chat: %w", err)
	}

	result, err := parseSummary(resp.Content)
	if err != nil {
		s.logger.Warn("summarizer: skipping malformed response", "error", err)
		return nil
	}

	if result.Summary != "" {
		if err := s.repo.UpdateConversationSummary(ctx, convID, result.Summary); err != nil {
			return fmt.Errorf("updating conversation summary: %w", err)
		}
	}
	if len(result.Observations) > 0 {
		if err := s.repo.SaveObservations(ctx, convID, result.Observations); err != nil {
			return fmt.Errorf("saving summary observations: %w", err)
		}
	}

	rows := AggregateUserModel(nil, result.Observations)
	if len(rows) > 0 {
		if err := s.repo.UpsertUserModel(ctx, rows); err != nil {
			return fmt.Errorf("upserting user model: %w", err)
		}
	}
	return nil
}

// summarizeMessages prefixes the system prompt to the conversation messages.
func summarizeMessages(msgs []core.Message) []core.Message {
	out := make([]core.Message, 0, len(msgs)+1)
	out = append(out, core.Message{Role: core.RoleSystem, Content: summarizerSystemPrompt})
	return append(out, msgs...)
}

// summaryResult is the parsed form of the summarizer's structured response.
type summaryResult struct {
	Summary      string
	Observations []core.Observation
}

// summaryJSON is the wire shape of the summarizer's response.
type summaryJSON struct {
	Summary      string            `json:"summary"`
	Observations []observationJSON `json:"observations"`
}

// parseSummary strips markdown fences and unmarshals the {summary,
// observations[]} object. A non-JSON response returns an error.
func parseSummary(content string) (summaryResult, error) {
	var raw summaryJSON
	if err := json.Unmarshal([]byte(stripFences(content)), &raw); err != nil {
		return summaryResult{}, fmt.Errorf("parsing summary: %w", err)
	}
	obs := make([]core.Observation, 0, len(raw.Observations))
	for _, o := range raw.Observations {
		obs = append(obs, o.toObservation())
	}
	return summaryResult{Summary: raw.Summary, Observations: obs}, nil
}
