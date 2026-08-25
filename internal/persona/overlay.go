package persona

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnknownPersonality is returned when /personality names a preset that is
// neither built-in nor configured.
var ErrUnknownPersonality = errors.New("unknown personality")

// clearNames are the aliases that remove an active overlay (spec PER-003).
var clearNames = map[string]bool{
	"": true, "none": true, "default": true, "neutral": true,
}

// builtinPersonalities ship with the binary. Values are appended to the
// identity slot as a session-scoped overlay.
var builtinPersonalities = map[string]string{
	"concise":   "Answer in as few words as possible. No preamble, no summary of the question. One-liners beat paragraphs; bullet lists beat prose.",
	"teacher":   "Teach while answering: after giving the answer, briefly explain why it works and name the underlying concept so the user learns the pattern, not just the fix.",
	"technical": "Precision over friendliness. Use exact terminology, include code or commands where relevant, skip pleasantries, and state assumptions explicitly.",
	"creative":  "Play with language: vivid metaphors and unexpected angles are welcome as long as the substance stays accurate.",
}

// Overlays resolves /personality names against built-in presets plus custom
// presets from config `agent.personalities`.
type Overlays struct {
	custom map[string]string
}

// NewOverlays returns an overlay resolver carrying the given custom presets.
func NewOverlays(custom map[string]string) *Overlays {
	return &Overlays{custom: custom}
}

// Resolve maps a personality name to its overlay text. Clearing names
// ("", none, default, neutral) return empty text with no error. Unknown
// names return ErrUnknownPersonality.
func (o *Overlays) Resolve(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if clearNames[key] {
		return "", nil
	}
	if text, ok := builtinPersonalities[key]; ok {
		return text, nil
	}
	if o != nil {
		if text, ok := o.custom[key]; ok {
			return strings.TrimSpace(text), nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownPersonality, name)
}

// Names lists every resolvable non-clearing personality name in plain
// alphabetical order. Used by status output.
func (o *Overlays) Names() []string {
	names := make([]string, 0, len(builtinPersonalities)+len(o.custom))
	for n := range builtinPersonalities {
		names = append(names, n)
	}
	if o != nil {
		for n := range o.custom {
			if _, dup := builtinPersonalities[n]; !dup {
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	return names
}
