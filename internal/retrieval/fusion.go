package retrieval

import (
	"sort"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
)

const rrfK = 60

// fuseWeightedRRF merges ranked lists with per-list weights using Reciprocal Rank Fusion.
func fuseWeightedRRF(lists [][]document.SearchHit, weights []float64, topK int) []document.SearchHit {
	if len(lists) == 0 {
		return nil
	}
	if len(weights) < len(lists) {
		w := make([]float64, len(lists))
		copy(w, weights)
		for i := len(weights); i < len(lists); i++ {
			w[i] = 1
		}
		weights = w
	}

	type agg struct {
		hit   document.SearchHit
		score float64
	}
	byChunk := make(map[uuid.UUID]*agg)

	for li, list := range lists {
		w := weights[li]
		if w <= 0 {
			continue
		}
		for rank, hit := range list {
			score := w / float64(rrfK+rank+1)
			if cur, ok := byChunk[hit.ChunkID]; ok {
				cur.score += score
				if hit.Score > cur.hit.Score {
					cur.hit = hit
				}
			} else {
				h := hit
				byChunk[hit.ChunkID] = &agg{hit: h, score: score}
			}
		}
	}

	out := make([]document.SearchHit, 0, len(byChunk))
	for _, a := range byChunk {
		h := a.hit
		h.Score = float32(a.score)
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ChunkID.String() < out[j].ChunkID.String()
		}
		return out[i].Score > out[j].Score
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}
