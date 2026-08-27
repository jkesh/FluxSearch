package settings

import (
	"fmt"
	"os"
	"strconv"
)

const (
	DefaultSearchRecallK              = 50
	DefaultMilvusSparseDropRatioBuild = 0.2
	DefaultRerankBatchSize            = 32

	SearchModeDense        = "dense"
	SearchModeSparseHybrid = "sparse_hybrid"
	SearchModeDenseBM25    = "dense_bm25"
)

func (s AppSettings) withSearchDefaults() AppSettings {
	out := s.withMilvusDefaults()
	if out.SearchRecallK <= 0 {
		out.SearchRecallK = DefaultSearchRecallK
	}
	if out.MilvusSparseDropRatioBuild < 0 {
		out.MilvusSparseDropRatioBuild = DefaultMilvusSparseDropRatioBuild
	}
	if out.MilvusSparseDropRatioSearch < 0 {
		out.MilvusSparseDropRatioSearch = 0
	}
	if out.RerankModel == "" {
		out.RerankModel = "bge-reranker-v2-m3"
	}
	out.SearchMode = NormalizeSearchMode(out.SearchMode, out.SearchHybridEnabled)
	if out.SearchDenseWeight < 0 {
		out.SearchDenseWeight = 0
	}
	if out.SearchBM25Weight < 0 {
		out.SearchBM25Weight = 0
	}
	if out.SearchDenseWeight == 0 && out.SearchBM25Weight == 0 {
		out.SearchDenseWeight = 0.5
		out.SearchBM25Weight = 0.5
	}
	// Keep Milvus sparse schema in sync when querying sparse hybrid.
	if out.SearchMode == SearchModeSparseHybrid {
		out.SearchHybridEnabled = true
	}
	return out
}

// EffectiveSearchMode returns the active retrieval backend.
func (s AppSettings) EffectiveSearchMode() string {
	return NormalizeSearchMode(s.SearchMode, s.SearchHybridEnabled)
}

// NormalizeSearchMode maps empty/legacy values to a canonical mode.
func NormalizeSearchMode(mode string, hybridEnabled bool) string {
	switch mode {
	case SearchModeDense, SearchModeSparseHybrid, SearchModeDenseBM25:
		return mode
	}
	if hybridEnabled {
		return SearchModeSparseHybrid
	}
	return SearchModeDense
}

// NormalizeFusionWeights clamps and fills defaults for dense+bm25 fusion.
func NormalizeFusionWeights(denseW, bm25W float64) (float64, float64) {
	if denseW < 0 {
		denseW = 0
	}
	if bm25W < 0 {
		bm25W = 0
	}
	if denseW == 0 && bm25W == 0 {
		return 0.5, 0.5
	}
	return denseW, bm25W
}

func (m *Manager) EffectiveSearchSettings() AppSettings {
	return m.Get().withSearchDefaults()
}

func (m *Manager) RerankAPIURL() string {
	if v := os.Getenv("FLUXSEARCH_RERANK_API_URL"); v != "" {
		return v
	}
	s := m.EffectiveSearchSettings()
	if s.RerankAPIURL != "" {
		return s.RerankAPIURL
	}
	if s.EmbeddingAPIURL != "" {
		return s.EmbeddingAPIURL
	}
	host := envOr("FLUXSEARCH_FLAGEMBEDDING_HOST", "127.0.0.1")
	port := envInt("FLUXSEARCH_FLAGEMBEDDING_PORT", 8091)
	return fmt.Sprintf("http://%s:%d/v1", host, port)
}

func (m *Manager) RerankBatchSize() int {
	if v := os.Getenv("FLUXSEARCH_RERANK_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultRerankBatchSize
}
