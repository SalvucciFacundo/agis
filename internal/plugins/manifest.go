// Package plugins manages external plugin discovery, manifest parsing,
// lifecycle management, and bridges tools and skills into AGIS registries.
package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9-_]+$`)

// Tool defines a single tool exported by a plugin.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Manifest defines the structure of a plugin.json file.
type Manifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Entrypoint  string   `json:"entrypoint,omitempty"`
	Tools       []Tool   `json:"tools,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// Validate checks that the manifest complies with schema requirements:
// - Name is required and matches ^[a-z0-9-_]+$
// - Version is required
// - Each declared tool has a non-empty name
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("plugin name is required")
	}
	if !nameRegex.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin name %q: must match ^[a-z0-9-_]+$", m.Name)
	}
	if m.Version == "" {
		return errors.New("plugin version is required")
	}
	for i, t := range m.Tools {
		if t.Name == "" {
			return fmt.Errorf("tool at index %d: tool name is required", i)
		}
	}
	return nil
}

// ParseManifest unmarshals and validates JSON data into a Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing plugin manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// ParseManifestFile reads a plugin.json file from disk and parses it.
func ParseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plugin manifest %s: %w", path, err)
	}
	return ParseManifest(data)
}
