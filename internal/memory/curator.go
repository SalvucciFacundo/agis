package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// curatorSystemPrompt instructs the provider to extract durable observations
// from the conversation as a bare JSON array. User-facing facts use the
// "user/" topic_key prefix so AggregateUserModel can separate them from
// project/technical notes.
const curatorSystemPrompt = `You are a memory curator. Extract durable facts, preferences, and decisions from the conversation into long-term memory observations.

Respond with a JSON array only. Each element has exactly these fields:
- "topic_key": a stable, namespaced key using "/" separators (for example "user/prefs/coffee" or "project/arch"). Facts about the user use the "user/" prefix.
- "type": a short category (for example "preference", "decision", "note").
- "content": the fact itself, concise.
- "importance": an integer from 1 (trivial) to 5 (critical).

If nothing is worth remembering, respond with []. Do not wrap the JSON in code fences.`

// Curator runs the LLM curation prompt over recent messages, parses the
// observations out of the structured response, and persists them.
type Curator struct {
	provider core.Provider
	repo     core.Repository
	logger   *slog.Logger
}

var _ core.Nudger = (*Curator)(nil)

// NewCurator returns a Curator backed by provider and repo. A nil logger falls
// back to slog.Default().
func NewCurator(provider core.Provider, repo core.Repository, logger *slog.Logger) *Curator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Curator{provider: provider, repo: repo, logger: logger}
}

// Nudge runs ONE Chat call asking the provider to extract observations, then
// persists them. Parse failures log and skip (returning nil error and no
// observations) so a malformed response never fails the turn; only genuine
// infrastructure failures (Chat or SaveObservations) return an error.
func (c *Curator) Nudge(ctx context.Context, convID string, msgs []core.Message) ([]core.Observation, error) {
	resp, err := c.provider.Chat(ctx, core.ChatRequest{Messages: curationMessages(msgs)})
	if err != nil {
		return nil, fmt.Errorf("curator chat: %w", err)
	}

	obs, err := parseObservations(resp.Content)
	if err != nil {
		c.logger.Warn("curator: skipping malformed response", "error", err)
		return nil, nil
	}
	if len(obs) == 0 {
		return nil, nil
	}

	if err := c.repo.SaveObservations(ctx, convID, obs); err != nil {
		return nil, fmt.Errorf("saving curated observations: %w", err)
	}
	return obs, nil
}

// curationMessages prefixes the system prompt to the conversation messages.
func curationMessages(msgs []core.Message) []core.Message {
	out := make([]core.Message, 0, len(msgs)+1)
	out = append(out, core.Message{Role: core.RoleSystem, Content: curatorSystemPrompt})
	return append(out, msgs...)
}

// observationJSON is the wire shape of one curated observation as the provider
// returns it. Importance is optional: zero (absent) defaults to 3 via
// clampImportance.
type observationJSON struct {
	TopicKey   string `json:"topic_key"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

// toObservation converts a wire observation into a domain observation,
// normalizing importance through the shared clamp (0 → 3, then [1,5]).
func (o observationJSON) toObservation() core.Observation {
	return core.Observation{
		TopicKey:   o.TopicKey,
		Type:       o.Type,
		Content:    o.Content,
		Importance: clampImportance(o.Importance),
	}
}

// parseObservations strips markdown fences and unmarshals the JSON array. A
// non-JSON (prose) response returns an error; the caller logs and skips.
func parseObservations(content string) ([]core.Observation, error) {
	var raw []observationJSON
	if err := json.Unmarshal([]byte(stripFences(content)), &raw); err != nil {
		return nil, fmt.Errorf("parsing observations: %w", err)
	}
	obs := make([]core.Observation, 0, len(raw))
	for _, r := range raw {
		obs = append(obs, r.toObservation())
	}
	return obs, nil
}

// stripFences removes a leading ```json (or bare ```) fence line and a
// trailing ``` fence, plus surrounding whitespace, so a prose-wrapped JSON
// array parses cleanly. Arbitrary prose is left as-is (and then fails JSON
// parsing in the caller).
func stripFences(content string) string {
	s := strings.TrimSpace(content)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = strings.TrimSpace(s[i+1:])
		} else {
			s = ""
		}
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
