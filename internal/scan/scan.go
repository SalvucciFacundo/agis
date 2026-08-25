// Package scan applies the fixed prompt-injection pattern list to text that
// will enter an LLM prompt from user-editable files (SOUL.md, persona
// overlays, skill contents). It is deliberately simple: lowercase substring
// matching per line, dropping flagged lines. Deterministic and fully tested —
// no heuristics, no NLP.
package scan

import "strings"

// patterns are lowercase substrings that mark a line as a prompt-injection
// attempt. Keep this list conservative: every entry here drops real content.
var patterns = []string{
	"ignore all previous",
	"ignore previous",
	"disregard all previous",
	"disregard your",
	"forget your instructions",
	"you are now",
	"act as if you have no",
	"reveal your instructions",
	"reveal your system prompt",
	"your system prompt is now",
}

// Lines removes every line of text containing a prompt-injection pattern
// (matched case-insensitively) and returns the cleaned text plus the number
// of dropped lines. Line endings are preserved for surviving lines; the input
// is otherwise untouched.
func Lines(text string) (string, int) {
	if text == "" {
		return "", 0
	}

	raw := strings.Split(text, "\n")
	clean := make([]string, 0, len(raw))
	dropped := 0

	for _, line := range raw {
		lower := strings.ToLower(line)
		injected := false
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				injected = true
				break
			}
		}
		if injected {
			dropped++
			continue
		}
		clean = append(clean, line)
	}

	return strings.Join(clean, "\n"), dropped
}
