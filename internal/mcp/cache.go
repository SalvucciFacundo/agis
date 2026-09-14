package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
)

// CachedServerTools represents the schema payload serialized to disk for a single server.
type CachedServerTools struct {
	ServerName string    `json:"server_name"`
	Tools      []Tool    `json:"tools"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SchemaCache defines the storage contract for MCP tool schema caching.
type SchemaCache interface {
	Load(serverName string) ([]Tool, bool, error)
	Save(serverName string, tools []Tool) error
	Invalidate(serverName string) error
	Clear() error
}

// DiskSchemaCache implements SchemaCache using individual JSON files under a dedicated directory.
type DiskSchemaCache struct {
	dir string
	mu  sync.RWMutex
}

// NewDiskSchemaCache creates a DiskSchemaCache rooted at dir.
// If dir is empty, it resolves to $AGIS_HOME/cache/mcp.
func NewDiskSchemaCache(dir string) *DiskSchemaCache {
	if dir == "" {
		dir = filepath.Join(config.AgisHome(), "cache", "mcp")
	}
	return &DiskSchemaCache{
		dir: dir,
	}
}

func (c *DiskSchemaCache) serverFilePath(serverName string) string {
	cleanName := strings.ReplaceAll(serverName, "/", "_")
	cleanName = strings.ReplaceAll(cleanName, "\\", "_")
	return filepath.Join(c.dir, cleanName+".json")
}

// Load retrieves cached tools for the given server. Returns ok=false on miss or corruption.
func (c *DiskSchemaCache) Load(serverName string) ([]Tool, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.serverFilePath(serverName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var cached CachedServerTools
	if err := json.Unmarshal(data, &cached); err != nil {
		// Tolerant recovery from corrupted cache
		return nil, false, nil
	}

	return cached.Tools, true, nil
}

// Save writes tools to disk atomically using a temp file and rename.
func (c *DiskSchemaCache) Save(serverName string, tools []Tool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.dir, 0700); err != nil {
		return fmt.Errorf("creating mcp cache directory: %w", err)
	}

	payload := CachedServerTools{
		ServerName: serverName,
		Tools:      tools,
		UpdatedAt:  time.Now().UTC(),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mcp tools cache: %w", err)
	}

	tmpFile, err := os.CreateTemp(c.dir, "mcp-cache-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for mcp cache: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing mcp cache payload: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("syncing mcp cache file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp mcp cache file: %w", err)
	}

	dest := c.serverFilePath(serverName)
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("committing mcp cache file: %w", err)
	}

	return nil
}

// Invalidate removes cached schema definitions for the given server.
func (c *DiskSchemaCache) Invalidate(serverName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.serverFilePath(serverName)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Clear removes all cached server schemas in the cache directory.
func (c *DiskSchemaCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			_ = os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}
	return nil
}
