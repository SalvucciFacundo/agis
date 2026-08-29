package memory_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// mockEmbedder implements core.Embedder for deterministic testing.
type mockEmbedder struct {
	mu          sync.Mutex
	dim         int
	embedFunc   func(ctx context.Context, text string) ([]float32, error)
	embedCalls  int
	batchCalls  int
	errToReturn error
}

func newMockEmbedder(dim int) *mockEmbedder {
	return &mockEmbedder{
		dim: dim,
	}
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	m.embedCalls++
	err := m.errToReturn
	fn := m.embedFunc
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if fn != nil {
		return fn(ctx, text)
	}
	return deterministicVector(text, m.dim), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	m.batchCalls++
	err := m.errToReturn
	fn := m.embedFunc
	m.mu.Unlock()

	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if fn != nil {
			v, err := fn(ctx, t)
			if err != nil {
				return nil, err
			}
			out[i] = v
		} else {
			out[i] = deterministicVector(t, m.dim)
		}
	}
	return out, nil
}

// deterministicVector generates a unit-like float32 vector based on keyword presence.
func deterministicVector(text string, dim int) []float32 {
	v := make([]float32, dim)
	lower := strings.ToLower(text)

	// Concept 1: automotive (car, automobile, vehicle, mechanic, repair, maintenance)
	if strings.Contains(lower, "car") || strings.Contains(lower, "automobile") ||
		strings.Contains(lower, "vehicle") || strings.Contains(lower, "repair") ||
		strings.Contains(lower, "maintenance") {
		v[0] = 0.9
		v[1] = 0.1
	}

	// Concept 2: cooking (cake, recipe, baking, food)
	if strings.Contains(lower, "cake") || strings.Contains(lower, "baking") ||
		strings.Contains(lower, "recipe") || strings.Contains(lower, "chocolate") {
		v[2] = 0.9
		v[3] = 0.1
	}

	// Concept 3: programming (golang, python, code, software)
	if strings.Contains(lower, "golang") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "software") || strings.Contains(lower, "programming") {
		v[4] = 0.9
		v[5] = 0.1
	}

	// Default non-zero fallback for any text
	if v[0] == 0 && v[2] == 0 && v[4] == 0 {
		v[dim-1] = 0.5
	}
	return v
}

func TestRepository_HybridSearch_NilEmbedder_FallsBackToFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	// Repository without embedder
	repo, err := memory.NewRepository(ctx, dbPath)
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	obs := []core.Observation{
		{TopicKey: "arch.go", Type: "architecture", Content: "Clean architecture in Go systems", Importance: 4},
		{TopicKey: "food.recipe", Type: "preference", Content: "Chocolate cake baking steps", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	require.NoError(t, err)

	// Search keyword present in FTS
	results, err := repo.Search(ctx, "architecture", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "Clean architecture")

	// Search keyword not present in FTS
	results, err = repo.Search(ctx, "microservices", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestRepository_HybridSearch_EmbedderError_FallsBackToFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)
	mock.errToReturn = errors.New("ollama connection refused")

	repo, err := memory.NewRepository(ctx, dbPath, memory.WithEmbedder(mock))
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	obs := []core.Observation{
		{TopicKey: "tech.go", Type: "fact", Content: "Golang concurrency with channels", Importance: 5},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	require.NoError(t, err)

	// Search should NOT return error; must transparently fall back to FTS5
	results, err := repo.Search(ctx, "channels", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "Golang concurrency")
}

func TestRepository_HybridSearch_TrueHybrid_SemanticAndFTS(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := memory.NewRepository(ctx, dbPath, memory.WithEmbedder(mock))
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	obs := []core.Observation{
		// Obs 1: Contains exact keyword "maintenance" AND has automotive semantic vector
		{TopicKey: "auto.maint", Type: "fact", Content: "Car maintenance checklist", Importance: 5},
		// Obs 2: Does NOT contain keyword "maintenance", but has automotive semantic vector ("automobile repair")
		{TopicKey: "auto.repair", Type: "fact", Content: "Automobile repair fundamentals", Importance: 4},
		// Obs 3: Irrelevant topic (cooking)
		{TopicKey: "food.cake", Type: "fact", Content: "Chocolate cake baking", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	require.NoError(t, err)

	// Flush async embedding queue
	repo.FlushEmbeddings(ctx)

	// Query: "maintenance"
	// FTS matches: Obs 1 ("Car maintenance checklist")
	// Vector matches: Obs 1 ("Car maintenance...") and Obs 2 ("Automobile repair...")
	// RRF should merge them:
	// Obs 1: rank 1 in FTS, rank 1/2 in Vector -> top rank
	// Obs 2: rank 2 in Vector, missing in FTS -> retrieved via semantic vector search!
	// Obs 3: neither matches
	results, err := repo.Search(ctx, "maintenance", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Contains(t, results[0].Content, "maintenance checklist")
	assert.Contains(t, results[1].Content, "Automobile repair")
}

func TestRepository_HybridSearch_ObservationUpdateReindexesVector(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := memory.NewRepository(ctx, dbPath, memory.WithEmbedder(mock))
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	// Initially: automotive topic
	obs1 := []core.Observation{
		{TopicKey: "dynamic.topic", Type: "fact", Content: "Automobile repair shop", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs1)
	require.NoError(t, err)
	repo.FlushEmbeddings(ctx)

	// Verify matches automotive search
	res, err := repo.Search(ctx, "car maintenance", 10)
	require.NoError(t, err)
	require.Len(t, res, 1)

	// Update topic to cooking concept
	obs2 := []core.Observation{
		{TopicKey: "dynamic.topic", Type: "fact", Content: "Chocolate cake baking recipe", Importance: 4},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs2)
	require.NoError(t, err)
	repo.FlushEmbeddings(ctx)

	// Search automotive -> should now return 0 results
	res, err = repo.Search(ctx, "car maintenance", 10)
	require.NoError(t, err)
	assert.Empty(t, res)

	// Search cooking -> should now return 1 result
	res, err = repo.Search(ctx, "cake recipe", 10)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Contains(t, res[0].Content, "Chocolate cake baking recipe")
}

func TestRepository_HybridSearch_LimitCap(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := memory.NewRepository(ctx, dbPath, memory.WithEmbedder(mock))
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	obs := []core.Observation{
		{TopicKey: "code.1", Type: "fact", Content: "Golang concurrency code", Importance: 3},
		{TopicKey: "code.2", Type: "fact", Content: "Golang memory optimization code", Importance: 3},
		{TopicKey: "code.3", Type: "fact", Content: "Golang software architecture code", Importance: 3},
	}
	err = repo.SaveObservations(ctx, conv.ID, obs)
	require.NoError(t, err)
	repo.FlushEmbeddings(ctx)

	results, err := repo.Search(ctx, "Golang code", 2)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestRepository_ConcurrentSearchAndIndexing_Race(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"

	mock := newMockEmbedder(8)

	repo, err := memory.NewRepository(ctx, dbPath, memory.WithEmbedder(mock))
	require.NoError(t, err)
	defer repo.Close()

	conv, err := repo.CreateConversation(ctx, "Session 1")
	require.NoError(t, err)

	var wg sync.WaitGroup
	workers := 8
	iterations := 25

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i%2 == 0 {
					_ = repo.SaveObservations(ctx, conv.ID, []core.Observation{
						{
							TopicKey:   fmt.Sprintf("worker.%d.topic.%d", workerID, i),
							Type:       "fact",
							Content:    fmt.Sprintf("Worker %d software code optimization step %d", workerID, i),
							Importance: 3,
						},
					})
				} else {
					_, _ = repo.Search(ctx, "software code", 5)
				}
			}
		}()
	}

	wg.Wait()
	repo.FlushEmbeddings(ctx)
}
