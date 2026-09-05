package llm

import (
	"sync"
)

// CredentialPool manages multiple API keys for an LLM provider with thread-safe
// reactive rotation and stampede prevention.
type CredentialPool struct {
	mu   sync.RWMutex
	keys []string
	idx  int
}

// NewCredentialPool creates a CredentialPool, deduplicating keys while preserving
// order. If primaryKey is non-empty, it is placed first if not already present.
func NewCredentialPool(primaryKey string, keys []string) *CredentialPool {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(keys)+1)

	if primaryKey != "" {
		seen[primaryKey] = struct{}{}
		unique = append(unique, primaryKey)
	}

	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, exists := seen[k]; !exists {
			seen[k] = struct{}{}
			unique = append(unique, k)
		}
	}

	return &CredentialPool{
		keys: unique,
		idx:  0,
	}
}

// CurrentKey returns the currently active API key, or empty string if the pool is empty.
func (p *CredentialPool) CurrentKey() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.keys) == 0 {
		return ""
	}
	return p.keys[p.idx]
}

// RotateKey performs reactive key rotation upon failure.
// If failedKey does not match the currently active key (another concurrent request
// already advanced the key), RotateKey returns the active key and true without advancing.
// If failedKey matches, it advances the active key. If the pool contains <= 1 key,
// it returns (currentKey, false) indicating no alternative keys exist.
func (p *CredentialPool) RotateKey(failedKey string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return "", false
	}
	if len(p.keys) == 1 {
		return p.keys[0], false
	}

	cur := p.keys[p.idx]
	if cur != failedKey {
		// Another goroutine already advanced the key past failedKey
		return cur, true
	}

	p.idx = (p.idx + 1) % len(p.keys)
	return p.keys[p.idx], true
}

// Len returns the number of unique credentials in the pool.
func (p *CredentialPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.keys)
}
