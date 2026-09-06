package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// ToolMetadata describes searchable summary attributes for a tool runner.
type ToolMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Backend     string `json:"backend"`
}

// ToolSearchRunner implements core.ToolRunner for dynamic tool searching.
type ToolSearchRunner struct {
	inventory    []core.ToolRunner
	getInventory func() []core.ToolRunner
}

// NewToolSearchRunner creates a ToolSearchRunner backed by a static inventory slice.
func NewToolSearchRunner(inventory []core.ToolRunner) *ToolSearchRunner {
	return &ToolSearchRunner{
		inventory: inventory,
	}
}

// NewToolSearchRunnerWithGetter creates a ToolSearchRunner backed by a dynamic inventory getter.
func NewToolSearchRunnerWithGetter(getter func() []core.ToolRunner) *ToolSearchRunner {
	return &ToolSearchRunner{
		getInventory: getter,
	}
}

// Backend implements core.ToolRunner.
func (t *ToolSearchRunner) Backend() string {
	return "internal"
}

// Name implements core.ToolRunner.
func (t *ToolSearchRunner) Name() string {
	return "tool_search"
}

// Description implements core.ToolRunner.
func (t *ToolSearchRunner) Description() string {
	return "Search for available tools by keyword or category when needed. Returns matching tool names and short descriptions."
}

// Run executes the search query and returns matching tool metadata serialized as JSON.
func (t *ToolSearchRunner) Run(_ context.Context, input string) (string, error) {
	var req struct {
		Query    string `json:"query"`
		Category string `json:"category"`
	}

	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("parsing search arguments: %w", err)
	}

	query := strings.TrimSpace(req.Query)
	category := strings.TrimSpace(req.Category)

	if query == "" && category == "" {
		return "", errors.New("at least one search parameter (query or category) is required")
	}

	runners := t.inventory
	if t.getInventory != nil {
		runners = t.getInventory()
	}

	matches := make([]ToolMetadata, 0)
	queryLower := strings.ToLower(query)
	categoryLower := strings.ToLower(category)

	for _, r := range runners {
		name := r.Name()
		if name == "" {
			name = "shell-" + r.Backend()
		}
		desc := r.Description()
		backend := r.Backend()
		cat := InferCategory(name, backend, desc)

		if c, ok := r.(interface{ Category() string }); ok && c.Category() != "" {
			cat = c.Category()
		}

		// Match category if specified
		if category != "" {
			matchCat := strings.EqualFold(cat, categoryLower) ||
				strings.EqualFold(backend, categoryLower) ||
				(categoryLower == "subagents" && (cat == "subagent" || cat == "subagents")) ||
				(categoryLower == "subagent" && (cat == "subagent" || cat == "subagents"))
			if !matchCat {
				continue
			}
		}

		// Match query if specified
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(name), queryLower)
			descMatch := strings.Contains(strings.ToLower(desc), queryLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		matches = append(matches, ToolMetadata{
			Name:        name,
			Description: desc,
			Category:    cat,
			Backend:     backend,
		})
	}

	data, err := json.Marshal(matches)
	if err != nil {
		return "", fmt.Errorf("marshaling search results: %w", err)
	}

	return string(data), nil
}

// InferCategory determines the functional category for a tool.
func InferCategory(name, backend, description string) string {
	lowerName := strings.ToLower(name)
	lowerBackend := strings.ToLower(backend)

	switch {
	case strings.HasPrefix(lowerBackend, "mcp") || strings.HasPrefix(lowerName, "mcp_"):
		return "mcp"
	case lowerBackend == "web" || strings.HasPrefix(lowerName, "web_"):
		return "web"
	case lowerBackend == "subagent" || lowerName == "delegate_task":
		return "subagents"
	case lowerBackend == "local" || lowerBackend == "docker" || lowerBackend == "ssh" || strings.HasPrefix(lowerName, "shell-"):
		return "shell"
	case lowerBackend == "internal" || lowerName == "tool_search" || lowerName == "load_tool":
		return "internal"
	default:
		if lowerBackend != "" {
			return lowerBackend
		}
		return "general"
	}
}
