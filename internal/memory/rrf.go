package memory

import (
	"sort"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// DefaultRRFK is the default smoothing constant (k = 60) for Reciprocal Rank Fusion.
const DefaultRRFK = 60.0

type rrfCandidate struct {
	result core.SearchResult
	score  float64
}

// ReciprocalRankFusion combines multiple ranked lists of SearchResult items into a single
// ranked list scored by RRF:
//
//	RRF(d) = sum(1 / (k + rank_i(d)))
//
// where rank_i(d) is the 1-based index of item d in ranked list i.
// Results are deduplicated by (DocType, DocID), sorted by RRF score descending,
// with ties deterministically broken by DocID ascending (then DocType ascending).
func ReciprocalRankFusion(rankedLists [][]core.SearchResult, k float64) []core.SearchResult {
	if k <= 0 {
		k = DefaultRRFK
	}

	type docKey struct {
		docType string
		docID   string
	}

	scores := make(map[docKey]*rrfCandidate)

	for _, list := range rankedLists {
		seenInList := make(map[docKey]bool)
		for rankIdx, item := range list {
			key := docKey{
				docType: item.DocType,
				docID:   item.DocID,
			}
			// Deduplicate within the same ranked list (keep first occurrence rank)
			if seenInList[key] {
				continue
			}
			seenInList[key] = true

			rank := float64(rankIdx + 1) // 1-based rank
			rrfContribution := 1.0 / (k + rank)

			cand, exists := scores[key]
			if !exists {
				cand = &rrfCandidate{
					result: item,
					score:  0,
				}
				scores[key] = cand
			}
			cand.score += rrfContribution
		}
	}

	if len(scores) == 0 {
		return []core.SearchResult{}
	}

	candidates := make([]*rrfCandidate, 0, len(scores))
	for _, cand := range scores {
		candidates = append(candidates, cand)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].result.DocID != candidates[j].result.DocID {
			return candidates[i].result.DocID < candidates[j].result.DocID
		}
		return candidates[i].result.DocType < candidates[j].result.DocType
	})

	results := make([]core.SearchResult, len(candidates))
	for i, cand := range candidates {
		results[i] = cand.result
	}

	return results
}
