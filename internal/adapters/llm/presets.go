package llm

import (
	"strings"
)

// Provider presets catalog mapping provider canonical names to their default base URLs.
var providerPresets = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"ollama":     "http://localhost:11434/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"gemini":     "https://generativelanguage.googleapis.com/v1beta/openai",
	"deepseek":   "https://api.deepseek.com/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"mistral":    "https://api.mistral.ai/v1",
	"xai":        "https://api.x.ai/v1",
	"together":   "https://api.together.xyz/v1",
	"cohere":     "https://api.cohere.com/v2",
	"anthropic":  "https://api.anthropic.com",
}

// ResolveBaseURL returns the explicit baseURL if non-empty; otherwise it resolves
// the default canonical endpoint for the given provider name. If the provider is
// unknown and baseURL is empty, it falls back to OpenAI's public endpoint.
func ResolveBaseURL(provider, baseURL string) string {
	if trimmed := strings.TrimSpace(baseURL); trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if url, ok := providerPresets[key]; ok {
		return strings.TrimRight(url, "/")
	}
	return openAIBaseURL
}
