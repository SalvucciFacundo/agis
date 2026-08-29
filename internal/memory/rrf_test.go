package memory_test

import (
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReciprocalRankFusion_EmptyInputs(t *testing.T) {
	t.Parallel()

	// Empty lists
	res := memory.ReciprocalRankFusion(nil, 60)
	assert.Empty(t, res)

	res = memory.ReciprocalRankFusion([][]core.SearchResult{}, 60)
	assert.Empty(t, res)

	// Lists containing empty slices
	res = memory.ReciprocalRankFusion([][]core.SearchResult{{}, {}}, 60)
	assert.Empty(t, res)
}

func TestReciprocalRankFusion_SingleList(t *testing.T) {
	t.Parallel()

	list := []core.SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "first"},
		{DocType: "observation", DocID: "obs-2", Content: "second"},
		{DocType: "message", DocID: "msg-1", Content: "third"},
	}

	res := memory.ReciprocalRankFusion([][]core.SearchResult{list}, 60)
	require.Len(t, res, 3)
	assert.Equal(t, "obs-1", res[0].DocID)
	assert.Equal(t, "obs-2", res[1].DocID)
	assert.Equal(t, "msg-1", res[2].DocID)
}

func TestReciprocalRankFusion_MultiListScoring(t *testing.T) {
	t.Parallel()

	// List 1 (e.g. FTS): D1 (rank 1), D2 (rank 2)
	// List 2 (e.g. Vector): D1 (rank 1), D3 (rank 2)
	// D1 is in both at rank 1: score = 1/61 + 1/61 = 2/61 ≈ 0.0327869
	// D2 is in list 1 rank 2: score = 1/62 ≈ 0.0161290
	// D3 is in list 2 rank 2: score = 1/62 ≈ 0.0161290
	// D2 and D3 tie on score. D2 ("obs-2") vs D3 ("obs-3") -> "obs-2" < "obs-3"

	fts := []core.SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "shared top"},
		{DocType: "observation", DocID: "obs-2", Content: "fts only"},
	}
	vec := []core.SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "shared top"},
		{DocType: "observation", DocID: "obs-3", Content: "vector only"},
	}

	res := memory.ReciprocalRankFusion([][]core.SearchResult{fts, vec}, 60)
	require.Len(t, res, 3)
	assert.Equal(t, "obs-1", res[0].DocID)
	assert.Equal(t, "obs-2", res[1].DocID)
	assert.Equal(t, "obs-3", res[2].DocID)
}

func TestReciprocalRankFusion_DeterministicTieBreaking(t *testing.T) {
	t.Parallel()

	// Two items appearing at the exact same rank in separate lists (same score)
	listA := []core.SearchResult{
		{DocType: "observation", DocID: "doc-b", Content: "beta"},
	}
	listB := []core.SearchResult{
		{DocType: "observation", DocID: "doc-a", Content: "alpha"},
	}

	res := memory.ReciprocalRankFusion([][]core.SearchResult{listA, listB}, 60)
	require.Len(t, res, 2)
	assert.Equal(t, "doc-a", res[0].DocID, "doc-a should precede doc-b on equal score due to doc_id sorting")
	assert.Equal(t, "doc-b", res[1].DocID)
}

func TestReciprocalRankFusion_Deduplication(t *testing.T) {
	t.Parallel()

	// Same doc repeated in same list
	list := []core.SearchResult{
		{DocType: "observation", DocID: "obs-1", Content: "first occurrence"},
		{DocType: "observation", DocID: "obs-1", Content: "second occurrence"},
	}

	res := memory.ReciprocalRankFusion([][]core.SearchResult{list}, 60)
	require.Len(t, res, 1)
	assert.Equal(t, "obs-1", res[0].DocID)
}
