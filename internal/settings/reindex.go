package settings

// ReindexPlan 配置变更后需要的重处理动作
type ReindexPlan struct {
	Needed             bool     `json:"needed"`
	RechunkAll         bool     `json:"rechunk_all"`
	ReembedAll         bool     `json:"reembed_all"`
	RecreateCollection bool     `json:"recreate_collection"`
	Reasons            []string `json:"reasons"`
}

func DetectReindexPlan(before, after AppSettings) ReindexPlan {
	before = before.withMilvusDefaults()
	after = after.withMilvusDefaults()

	var plan ReindexPlan
	var reasons []string

	if before.ChunkMaxTokens != after.ChunkMaxTokens || before.ChunkOverlapTokens != after.ChunkOverlapTokens {
		plan.RechunkAll = true
		reasons = append(reasons, "分块参数变更")
	}

	if embeddingVectorSpaceChanged(before, after) {
		plan.ReembedAll = true
		reasons = append(reasons, "Embedding 模型/维度变更")
		if before.EmbeddingDim != after.EmbeddingDim {
			plan.RecreateCollection = true
		}
	}

	if before.EmbeddingMaxLength != after.EmbeddingMaxLength {
		plan.ReembedAll = true
		reasons = append(reasons, "Embedding max_length 变更")
	}

	if milvusStructureChanged(before, after) {
		plan.RecreateCollection = true
		if !plan.RechunkAll {
			plan.ReembedAll = true
		}
		reasons = append(reasons, "Milvus 索引结构变更")
	}

	if before.SearchHybridEnabled != after.SearchHybridEnabled {
		plan.RecreateCollection = true
		if !plan.RechunkAll {
			plan.ReembedAll = true
		}
		reasons = append(reasons, "Hybrid 检索开关变更")
	}

	if plan.RechunkAll {
		plan.ReembedAll = false
	}

	plan.Reasons = reasons
	plan.Needed = plan.RechunkAll || plan.ReembedAll || plan.RecreateCollection
	return plan
}

func embeddingVectorSpaceChanged(before, after AppSettings) bool {
	return before.EmbeddingProvider != after.EmbeddingProvider ||
		before.EmbeddingModel != after.EmbeddingModel ||
		before.EmbeddingDim != after.EmbeddingDim ||
		before.EmbeddingAPIURL != after.EmbeddingAPIURL ||
		before.EmbeddingLocalBackend != after.EmbeddingLocalBackend
}

func milvusStructureChanged(before, after AppSettings) bool {
	before = before.withMilvusDefaults().withSearchDefaults()
	after = after.withMilvusDefaults().withSearchDefaults()
	return before.MilvusIndexType != after.MilvusIndexType ||
		before.MilvusMetric != after.MilvusMetric ||
		before.MilvusNList != after.MilvusNList ||
		before.MilvusHNSWM != after.MilvusHNSWM ||
		before.MilvusHNSWEfConstruction != after.MilvusHNSWEfConstruction ||
		before.MilvusSparseDropRatioBuild != after.MilvusSparseDropRatioBuild
}

func (s AppSettings) withMilvusDefaults() AppSettings {
	out := s
	if out.MilvusIndexType == "" {
		out.MilvusIndexType = "ivf_flat"
	}
	if out.MilvusMetric == "" {
		out.MilvusMetric = "IP"
	}
	if out.MilvusNList <= 0 {
		out.MilvusNList = 128
	}
	if out.MilvusNProbe <= 0 {
		out.MilvusNProbe = 16
	}
	if out.MilvusHNSWM <= 0 {
		out.MilvusHNSWM = 16
	}
	if out.MilvusHNSWEfConstruction <= 0 {
		out.MilvusHNSWEfConstruction = 200
	}
	if out.MilvusHNSWEf <= 0 {
		out.MilvusHNSWEf = 64
	}
	if out.SearchTopK <= 0 {
		out.SearchTopK = 5
	}
	return out
}
