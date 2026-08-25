package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// creatorSystemPrompt instructs the provider to distill a reusable procedure
// from the conversation, or to answer null when nothing durable emerged.
const creatorSystemPrompt = `You are a skill curator. Decide whether this conversation produced a REUSABLE procedure — a set of steps or instructions that would help with similar tasks in the future.

If it did, respond with a single JSON object:
{"name": "short-kebab-name", "description": "one line", "trigger": "optional keyword", "content": "the full step-by-step instructions"}

If nothing reusable emerged, respond with null.

Do not wrap the JSON in code fences.`

// Creator extracts agent-authored skills from a finished session (spec
// SKL-004). It mirrors memory.Curator: ONE Chat call per close, fenced-JSON
// parsing, malformed responses log-and-skip so closing never fails on a bad
// provider answer.
type Creator struct {
	provider core.Provider
	repo     core.Repository
	enabled  bool
	logger   *slog.Logger
}

// NewCreator returns a Creator. A disabled Creator is a no-op Extract.
func NewCreator(provider core.Provider, repo core.Repository, enabled bool, logger *slog.Logger) *Creator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Creator{provider: provider, repo: repo, enabled: enabled, logger: logger}
}

// Extract runs one Chat call over the session messages. It returns the saved
// agent skill, or nil when nothing qualified / the response was malformed /
// creation is disabled. Infrastructure errors (Chat, persistence) are
// returned for CloseSession's non-fatal logging.
func (c *Creator) Extract(ctx context.Context, convID string, msgs []core.Message) (*core.Skill, error) {
	if !c.enabled {
		return nil, nil
	}

	resp, err := c.provider.Chat(ctx, core.ChatRequest{Messages: creatorMessages(msgs)})
	if err != nil {
		return nil, fmt.Errorf("skill creator chat: %w", err)
	}

	skill := parseCreated(resp.Content)
	if skill == nil {
		c.logger.Warn("skills: no reusable procedure extracted", "convID", convID)
		return nil, nil
	}
	skill.Source = core.SourceAgent

	if err := c.repo.SaveSkill(ctx, *skill); err != nil {
		return nil, fmt.Errorf("saving created skill %q: %w", skill.Name, err)
	}
	payload := fmt.Sprintf(`{"name":%q}`, skill.Name)
	if err := c.repo.RecordSessionEvent(ctx, convID, "skill", payload); err != nil {
		return nil, fmt.Errorf("recording skill event for %q: %w", skill.Name, err)
	}
	return skill, nil
}

func creatorMessages(msgs []core.Message) []core.Message {
	out := make([]core.Message, 0, len(msgs)+1)
	out = append(out, core.Message{Role: core.RoleSystem, Content: creatorSystemPrompt})
	return append(out, msgs...)
}

// skillJSON is the wire shape of a created skill.
type skillJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"`
	Content     string `json:"content"`
}

// parseCreated strips fences and unmarshals the response. A literal null, an
// empty object, or missing required fields yield nil (nothing to save).
func parseCreated(content string) *core.Skill {
	var raw *skillJSON
	if err := json.Unmarshal([]byte(stripFences(content)), &raw); err != nil || raw == nil {
		return nil
	}
	name := strings.TrimSpace(raw.Name)
	desc := strings.TrimSpace(raw.Description)
	body := strings.TrimSpace(raw.Content)
	if name == "" || desc == "" || body == "" {
		return nil
	}
	return &core.Skill{
		Name:        name,
		Description: desc,
		Trigger:     strings.TrimSpace(raw.Trigger),
		Content:     body,
	}
}

// stripFences removes a leading ``` fence line and a trailing fence plus
// surrounding whitespace. Local copy of the memory-package helper: the two
// adapter packages must not import each other.
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
