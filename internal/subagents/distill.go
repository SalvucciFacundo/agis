// Package subagents implements isolated child brain delegation and knowledge distillation.
package subagents

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

var (
	headingRegex   = regexp.MustCompile(`(?i)^##\s+(key\s+learnings|discoveries|key\s+takeaways|decisions)\s*:?\s*$`)
	orderedRegex   = regexp.MustCompile(`^[0-9]+\.\s+(.*)$`)
	unorderedRegex = regexp.MustCompile(`^[-*]\s+(.*)$`)
	slugCharRegex  = regexp.MustCompile(`[^a-z0-9]+`)
)

// ExtractKeyLearnings parses output text for markdown sections containing structured learning points.
// Extracted observations are namespaced by task slug and constrained by maxObs (clamped to [1, 5]).
func ExtractKeyLearnings(task string, output string, maxObs int, convID string) []core.Observation {
	effectiveMax := maxObs
	if effectiveMax <= 0 {
		effectiveMax = 3
	} else if effectiveMax > 5 {
		effectiveMax = 5
	}

	slug := sanitizeTaskSlug(task)
	lines := strings.Split(output, "\n")

	var (
		inSection   bool
		currentType string
		obs         = make([]core.Observation, 0, effectiveMax)
	)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			match := headingRegex.FindStringSubmatch(trimmed)
			if len(match) > 1 {
				inSection = true
				headerName := strings.ToLower(strings.TrimSpace(match[1]))
				if strings.Contains(headerName, "decision") {
					currentType = "decision"
				} else {
					currentType = "discovery"
				}
			} else {
				inSection = false
			}
			continue
		}

		if !inSection {
			continue
		}

		var itemText string
		if sub := orderedRegex.FindStringSubmatch(trimmed); len(sub) > 1 {
			itemText = strings.TrimSpace(sub[1])
		} else if sub := unorderedRegex.FindStringSubmatch(trimmed); len(sub) > 1 {
			itemText = strings.TrimSpace(sub[1])
		}

		if itemText == "" {
			continue
		}

		obs = append(obs, core.Observation{
			TopicKey:   fmt.Sprintf("subagent/%s/%d", slug, len(obs)+1),
			Type:       currentType,
			Content:    itemText,
			Importance: 3,
			SourceRef:  convID,
		})

		if len(obs) >= effectiveMax {
			break
		}
	}

	return obs
}

// sanitizeTaskSlug converts a prompt into a max 32-char lowercase alphanumeric-and-hyphen slug.
func sanitizeTaskSlug(task string) string {
	lowered := strings.ToLower(strings.TrimSpace(task))
	replaced := slugCharRegex.ReplaceAllString(lowered, "-")
	trimmed := strings.Trim(replaced, "-")

	if len(trimmed) > 32 {
		trimmed = strings.Trim(trimmed[:32], "-")
	}

	if trimmed == "" {
		return "task"
	}
	return trimmed
}
