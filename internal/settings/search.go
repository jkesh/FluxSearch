package settings

const (
	DefaultSearchRecallK              = 50
	DefaultMilvusSparseDropRatioBuild = 0.2
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
	return out
}

func (m *Manager) EffectiveSearchSettings() AppSettings {
	return m.Get().withSearchDefaults()
}

func (m *Manager) RerankAPIURL() string {
	s := m.EffectiveSearchSettings()
	if s.RerankAPIURL != "" {
		return s.RerankAPIURL
	}
	if s.EmbeddingAPIURL != "" {
		return s.EmbeddingAPIURL
	}
	return "http://127.0.0.1:8091/v1"
}
