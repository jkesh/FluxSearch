package settings

import milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"

func (m *Manager) MilvusIndexConfig() milvusstore.IndexConfig {
	s := m.Get().withMilvusDefaults().withSearchDefaults()
	return milvusstore.IndexConfig{
		IndexType:               s.MilvusIndexType,
		Metric:                  s.MilvusMetric,
		NList:                   s.MilvusNList,
		NProbe:                  s.MilvusNProbe,
		HNSWM:                   s.MilvusHNSWM,
		HNSWEfConstruction:      s.MilvusHNSWEfConstruction,
		HNSWEf:                  s.MilvusHNSWEf,
		ScoreThreshold:          float32(s.SearchScoreThreshold),
		HybridEnabled:           s.SearchHybridEnabled,
		SparseDropRatioBuild:    s.MilvusSparseDropRatioBuild,
		SparseDropRatioSearch:   s.MilvusSparseDropRatioSearch,
	}.Normalized()
}
