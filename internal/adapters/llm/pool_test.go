package llm

import (
	"sync"
	"testing"
)

func TestNewCredentialPool_DeduplicationAndOrder(t *testing.T) {
	tests := []struct {
		name       string
		primaryKey string
		keys       []string
		wantKeys   []string
		wantLen    int
		wantCur    string
	}{
		{
			name:       "deduplicates and preserves order",
			primaryKey: "key-A",
			keys:       []string{"key-B", "key-A", "key-C", "key-B"},
			wantKeys:   []string{"key-A", "key-B", "key-C"},
			wantLen:    3,
			wantCur:    "key-A",
		},
		{
			name:       "empty primary with keys",
			primaryKey: "",
			keys:       []string{"k1", "k2", "k1"},
			wantKeys:   []string{"k1", "k2"},
			wantLen:    2,
			wantCur:    "k1",
		},
		{
			name:       "single key backward compatibility",
			primaryKey: "key-only",
			keys:       nil,
			wantKeys:   []string{"key-only"},
			wantLen:    1,
			wantCur:    "key-only",
		},
		{
			name:       "empty pool",
			primaryKey: "",
			keys:       nil,
			wantKeys:   []string{},
			wantLen:    0,
			wantCur:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewCredentialPool(tt.primaryKey, tt.keys)
			if pool.Len() != tt.wantLen {
				t.Errorf("pool.Len() = %d, want %d", pool.Len(), tt.wantLen)
			}
			if pool.CurrentKey() != tt.wantCur {
				t.Errorf("pool.CurrentKey() = %q, want %q", pool.CurrentKey(), tt.wantCur)
			}
		})
	}
}

func TestCredentialPool_RotateKey(t *testing.T) {
	t.Run("single key returns false", func(t *testing.T) {
		pool := NewCredentialPool("k1", nil)
		key, ok := pool.RotateKey("k1")
		if ok {
			t.Errorf("RotateKey on single key got ok = true, want false")
		}
		if key != "k1" {
			t.Errorf("RotateKey = %q, want 'k1'", key)
		}
	})

	t.Run("empty pool returns false", func(t *testing.T) {
		pool := NewCredentialPool("", nil)
		key, ok := pool.RotateKey("")
		if ok {
			t.Errorf("RotateKey on empty pool got ok = true, want false")
		}
		if key != "" {
			t.Errorf("RotateKey = %q, want ''", key)
		}
	})

	t.Run("multi key sequential rotation", func(t *testing.T) {
		pool := NewCredentialPool("k1", []string{"k2", "k3"})

		// Initial key is k1
		if pool.CurrentKey() != "k1" {
			t.Fatalf("initial key = %q, want 'k1'", pool.CurrentKey())
		}

		// Rotate k1 -> advances to k2
		k, ok := pool.RotateKey("k1")
		if !ok || k != "k2" {
			t.Fatalf("RotateKey(k1) = (%q, %v), want ('k2', true)", k, ok)
		}
		if pool.CurrentKey() != "k2" {
			t.Errorf("CurrentKey() = %q, want 'k2'", pool.CurrentKey())
		}

		// Stale rotation with k1 returns current key without advancing
		k, ok = pool.RotateKey("k1")
		if !ok || k != "k2" {
			t.Fatalf("stale RotateKey(k1) = (%q, %v), want ('k2', true)", k, ok)
		}
		if pool.CurrentKey() != "k2" {
			t.Errorf("CurrentKey() = %q, want 'k2'", pool.CurrentKey())
		}

		// Rotate k2 -> advances to k3
		k, ok = pool.RotateKey("k2")
		if !ok || k != "k3" {
			t.Fatalf("RotateKey(k2) = (%q, %v), want ('k3', true)", k, ok)
		}
		if pool.CurrentKey() != "k3" {
			t.Errorf("CurrentKey() = %q, want 'k3'", pool.CurrentKey())
		}
	})
}

func TestCredentialPool_ConcurrentRotation_StampedePrevention(t *testing.T) {
	pool := NewCredentialPool("k1", []string{"k2", "k3"})

	const concurrency = 50
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make([]string, concurrency)
	oks := make([]bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], oks[idx] = pool.RotateKey("k1")
		}(i)
	}

	wg.Wait()

	// Every goroutine that called RotateKey("k1") should receive ("k2", true)
	for i := 0; i < concurrency; i++ {
		if !oks[i] {
			t.Errorf("goroutine %d: ok = false, want true", i)
		}
		if results[i] != "k2" {
			t.Errorf("goroutine %d: key = %q, want 'k2'", i, results[i])
		}
	}

	// Current key must be k2 (advanced exactly once, not to k3 or wrap)
	if pool.CurrentKey() != "k2" {
		t.Errorf("after concurrent rotation, CurrentKey = %q, want 'k2'", pool.CurrentKey())
	}
}
