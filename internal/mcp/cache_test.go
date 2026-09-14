package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskSchemaCache_HitMissAndInvalidate(t *testing.T) {
	dir := t.TempDir()
	cache := NewDiskSchemaCache(dir)

	// 1. Miss on non-existent server
	tools, ok, err := cache.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load(nonexistent) error = %v", err)
	}
	if ok || len(tools) != 0 {
		t.Fatalf("Load(nonexistent) ok = %v, tools = %+v, want false and empty", ok, tools)
	}

	// 2. Save tools
	sampleTools := []Tool{
		{
			Name:        "echo",
			Description: "Echo input text",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		{
			Name:        "add",
			Description: "Add numbers",
		},
	}
	if err := cache.Save("my-server", sampleTools); err != nil {
		t.Fatalf("Save(my-server) error = %v", err)
	}

	// 3. Hit on saved server
	tools, ok, err = cache.Load("my-server")
	if err != nil {
		t.Fatalf("Load(my-server) error = %v", err)
	}
	if !ok || len(tools) != 2 {
		t.Fatalf("Load(my-server) ok = %v, tools count = %d, want true and 2", ok, len(tools))
	}
	if tools[0].Name != "echo" || tools[1].Name != "add" {
		t.Errorf("tools mismatch: %+v", tools)
	}

	// 4. Invalidate server
	if err := cache.Invalidate("my-server"); err != nil {
		t.Fatalf("Invalidate(my-server) error = %v", err)
	}

	tools, ok, err = cache.Load("my-server")
	if err != nil {
		t.Fatalf("Load after Invalidate error = %v", err)
	}
	if ok || len(tools) != 0 {
		t.Errorf("Load after Invalidate ok = %v, want false", ok)
	}
}

func TestDiskSchemaCache_CorruptedFileRecovery(t *testing.T) {
	dir := t.TempDir()
	cache := NewDiskSchemaCache(dir)

	targetFile := filepath.Join(dir, "corrupt-srv.json")
	if err := os.WriteFile(targetFile, []byte("{invalid-json"), 0600); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	// Loading corrupted file should gracefully return ok=false without crashing
	tools, ok, err := cache.Load("corrupt-srv")
	if err != nil {
		t.Fatalf("Load(corrupt-srv) returned unexpected error: %v", err)
	}
	if ok || len(tools) != 0 {
		t.Errorf("Load(corrupt-srv) ok = %v, want false for corrupt data", ok)
	}
}

func TestDiskSchemaCache_Clear(t *testing.T) {
	dir := t.TempDir()
	cache := NewDiskSchemaCache(dir)

	_ = cache.Save("srv1", []Tool{{Name: "t1"}})
	_ = cache.Save("srv2", []Tool{{Name: "t2"}})

	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	_, ok1, _ := cache.Load("srv1")
	_, ok2, _ := cache.Load("srv2")
	if ok1 || ok2 {
		t.Errorf("Clear() did not remove entries: ok1=%v, ok2=%v", ok1, ok2)
	}
}
