package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/fluxsearch/fluxsearch/internal/config"
)

const maskedSecret = "********"

// AppSettings 可通过 UI 动态修改的运行时配置（持久化到 app.settings.json）
type AppSettings struct {
	MonitorURL string `json:"monitor_url"`

	EmbeddingProvider     string `json:"embedding_provider"`
	EmbeddingLocalBackend string `json:"embedding_local_backend"`
	EmbeddingAPIURL       string `json:"embedding_api_url"`
	EmbeddingAPIKey       string `json:"embedding_api_key,omitempty"`
	EmbeddingModel        string `json:"embedding_model"`
	EmbeddingDim          int    `json:"embedding_dim"`
	EmbeddingBatchSize    int    `json:"embedding_batch_size"`
	EmbeddingMaxLength    int    `json:"embedding_max_length"`

	LLMProvider     string  `json:"llm_provider"`
	LLMLocalBackend string  `json:"llm_local_backend"`
	LLMAPIURL       string  `json:"llm_api_url"`
	LLMAPIKey       string  `json:"llm_api_key,omitempty"`
	LLMModel        string  `json:"llm_model"`
	LLMTemperature  float64 `json:"llm_temperature"`
	LLMMaxTokens    int     `json:"llm_max_tokens"`

	ChunkMaxTokens     int `json:"chunk_max_tokens"`
	ChunkOverlapTokens int `json:"chunk_overlap_tokens"`

	SearchTopK           int     `json:"search_top_k"`
	SearchScoreThreshold float64 `json:"search_score_threshold"`
	SearchHybridEnabled  bool    `json:"search_hybrid_enabled"`
	SearchRecallK        int     `json:"search_recall_k"`
	SearchRerankEnabled  bool    `json:"search_rerank_enabled"`
	RerankAPIURL         string  `json:"rerank_api_url"`
	RerankModel          string  `json:"rerank_model"`

	MilvusIndexType          string `json:"milvus_index_type"`
	MilvusMetric             string `json:"milvus_metric"`
	MilvusNList              int    `json:"milvus_nlist"`
	MilvusNProbe             int    `json:"milvus_nprobe"`
	MilvusHNSWM              int    `json:"milvus_hnsw_m"`
	MilvusHNSWEfConstruction int    `json:"milvus_hnsw_ef_construction"`
	MilvusHNSWEf             int    `json:"milvus_hnsw_ef"`
	MilvusSparseDropRatioBuild  float64 `json:"milvus_sparse_drop_ratio_build"`
	MilvusSparseDropRatioSearch float64 `json:"milvus_sparse_drop_ratio_search"`

	DocumentDedupEnabled       bool   `json:"document_dedup_enabled"`
	DocumentDedupMode          string `json:"document_dedup_mode"`
	DocumentDedupByContentHash bool   `json:"document_dedup_by_content_hash"`
	DocumentDedupBySourceURI   bool   `json:"document_dedup_by_source_uri"`
	ChunkDedupEnabled          bool   `json:"chunk_dedup_enabled"`
	ChunkDedupScope            string `json:"chunk_dedup_scope"`
}

// PublicView 返回给前端的视图（密钥脱敏）
type PublicView struct {
	AppSettings
	EmbeddingAPIKeySet bool   `json:"embedding_api_key_set"`
	LLMAPIKeySet       bool   `json:"llm_api_key_set"`
	SettingsPath       string `json:"settings_path"`
	EmbeddingReady     bool         `json:"embedding_ready"`
	EmbeddingStatus    string       `json:"embedding_status"`
	Reindex            ReindexView  `json:"reindex"`
}

type ReindexView struct {
	Running   bool   `json:"running"`
	Total     int    `json:"total"`
	Done      int    `json:"done"`
	Failed    int    `json:"failed"`
	LastError string `json:"last_error,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Manager struct {
	mu   sync.RWMutex
	path string
	data AppSettings
}

func NewManager() *Manager {
	m := &Manager{path: resolveSettingsPath()}
	m.data = m.defaultsFromEnv()
	if err := m.loadFromFile(); err != nil && !os.IsNotExist(err) {
		// 文件损坏时仍使用 env 默认值
	}
	m.applyToEnv(m.data)
	return m
}

func resolveSettingsPath() string {
	candidates := []string{
		"config/local/app.settings.json",
		"../config/local/app.settings.json",
		filepath.Join("..", "..", "config", "local", "app.settings.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) defaultsFromEnv() AppSettings {
	cfg := config.Load()
	d := defaultDedupSettings()
	return AppSettings{
		MonitorURL: envOr("FLUXSEARCH_MONITOR_URL", ""),

		EmbeddingProvider:     cfg.EmbeddingProvider,
		EmbeddingLocalBackend: cfg.EmbeddingLocalBackend,
		EmbeddingAPIURL:       cfg.EmbeddingAPIURL,
		EmbeddingAPIKey:       cfg.EmbeddingAPIKey,
		EmbeddingModel:        cfg.EmbeddingModel,
		EmbeddingDim:          nonZeroInt(cfg.EmbeddingDim, config.DefaultEmbeddingDim),
		EmbeddingBatchSize:    nonZeroInt(cfg.EmbeddingBatchSize, config.DefaultEmbeddingBatch),
		EmbeddingMaxLength:    nonZeroInt(cfg.EmbeddingMaxLength, config.DefaultEmbeddingMaxLength),

		LLMProvider:     envOr("FLUXSEARCH_LLM_PROVIDER", ""),
		LLMLocalBackend: envOr("FLUXSEARCH_LLM_LOCAL_BACKEND", "ollama"),
		LLMAPIURL:       envOr("FLUXSEARCH_LLM_API_URL", ""),
		LLMAPIKey:       envOr("FLUXSEARCH_LLM_API_KEY", ""),
		LLMModel:        envOr("FLUXSEARCH_LLM_MODEL", ""),
		LLMTemperature:  envFloat("FLUXSEARCH_LLM_TEMPERATURE", 0.7),
		LLMMaxTokens:    envInt("FLUXSEARCH_LLM_MAX_TOKENS", 2048),

		ChunkMaxTokens:     nonZeroInt(cfg.ChunkMaxTokens, config.DefaultChunkMaxTokens),
		ChunkOverlapTokens: cfg.ChunkOverlapTokens,

		SearchTopK:           envInt("FLUXSEARCH_SEARCH_TOP_K", 5),
		SearchScoreThreshold: envFloat("FLUXSEARCH_SEARCH_SCORE_THRESHOLD", 0),
		SearchHybridEnabled:  envBool("FLUXSEARCH_SEARCH_HYBRID_ENABLED", false),
		SearchRecallK:        envInt("FLUXSEARCH_SEARCH_RECALL_K", DefaultSearchRecallK),
		SearchRerankEnabled:  envBool("FLUXSEARCH_SEARCH_RERANK_ENABLED", false),
		RerankModel:          envOr("FLUXSEARCH_RERANK_MODEL", "bge-reranker-v2-m3"),

		MilvusIndexType:          envOr("FLUXSEARCH_MILVUS_INDEX_TYPE", "ivf_flat"),
		MilvusMetric:             envOr("FLUXSEARCH_MILVUS_METRIC", "IP"),
		MilvusNList:              envInt("FLUXSEARCH_MILVUS_NLIST", 128),
		MilvusNProbe:             envInt("FLUXSEARCH_MILVUS_NPROBE", 16),
		MilvusHNSWM:              envInt("FLUXSEARCH_MILVUS_HNSW_M", 16),
		MilvusHNSWEfConstruction: envInt("FLUXSEARCH_MILVUS_HNSW_EF_CONSTRUCTION", 200),
		MilvusHNSWEf:             envInt("FLUXSEARCH_MILVUS_HNSW_EF", 64),
		MilvusSparseDropRatioBuild:  envFloat("FLUXSEARCH_MILVUS_SPARSE_DROP_RATIO_BUILD", DefaultMilvusSparseDropRatioBuild),
		MilvusSparseDropRatioSearch: envFloat("FLUXSEARCH_MILVUS_SPARSE_DROP_RATIO_SEARCH", 0),

		DocumentDedupEnabled:       d.DocumentDedupEnabled,
		DocumentDedupMode:          d.DocumentDedupMode,
		DocumentDedupByContentHash: d.DocumentDedupByContentHash,
		DocumentDedupBySourceURI:   d.DocumentDedupBySourceURI,
		ChunkDedupEnabled:          d.ChunkDedupEnabled,
		ChunkDedupScope:            d.ChunkDedupScope,
	}
}

func (m *Manager) loadFromFile() error {
	raw, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	var file AppSettings
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	// 兼容旧版 chunk_max_chars / chunk_overlap
	var legacy struct {
		ChunkMaxChars  int `json:"chunk_max_chars"`
		ChunkOverlap   int `json:"chunk_overlap"`
	}
	if err := json.Unmarshal(raw, &legacy); err == nil {
		if file.ChunkMaxTokens <= 0 && legacy.ChunkMaxChars > 0 {
			file.ChunkMaxTokens = config.CharsToTokens(legacy.ChunkMaxChars)
		}
		if file.ChunkOverlapTokens == 0 && legacy.ChunkOverlap > 0 {
			file.ChunkOverlapTokens = config.CharsToTokens(legacy.ChunkOverlap)
		}
	}
	m.mergeFile(&file)
	return nil
}

func (m *Manager) mergeFile(file *AppSettings) {
	m.data.MonitorURL = file.MonitorURL

	if file.EmbeddingProvider != "" {
		m.data.EmbeddingProvider = file.EmbeddingProvider
	}
	if file.EmbeddingLocalBackend != "" {
		m.data.EmbeddingLocalBackend = file.EmbeddingLocalBackend
	}
	if file.EmbeddingAPIURL != "" {
		m.data.EmbeddingAPIURL = file.EmbeddingAPIURL
	}
	if file.EmbeddingAPIKey != "" {
		m.data.EmbeddingAPIKey = file.EmbeddingAPIKey
	}
	if file.EmbeddingModel != "" {
		m.data.EmbeddingModel = file.EmbeddingModel
	}
	if file.EmbeddingDim > 0 {
		m.data.EmbeddingDim = file.EmbeddingDim
	}
	if file.EmbeddingBatchSize > 0 {
		m.data.EmbeddingBatchSize = file.EmbeddingBatchSize
	}
	if file.EmbeddingMaxLength > 0 {
		m.data.EmbeddingMaxLength = file.EmbeddingMaxLength
	}

	if file.LLMProvider != "" {
		m.data.LLMProvider = file.LLMProvider
	}
	if file.LLMLocalBackend != "" {
		m.data.LLMLocalBackend = file.LLMLocalBackend
	}
	if file.LLMAPIURL != "" {
		m.data.LLMAPIURL = file.LLMAPIURL
	}
	if file.LLMAPIKey != "" {
		m.data.LLMAPIKey = file.LLMAPIKey
	}
	if file.LLMModel != "" {
		m.data.LLMModel = file.LLMModel
	}
	if file.LLMTemperature > 0 {
		m.data.LLMTemperature = file.LLMTemperature
	}
	if file.LLMMaxTokens > 0 {
		m.data.LLMMaxTokens = file.LLMMaxTokens
	}

	if file.ChunkMaxTokens > 0 {
		m.data.ChunkMaxTokens = file.ChunkMaxTokens
	}
	if file.ChunkOverlapTokens >= 0 {
		m.data.ChunkOverlapTokens = file.ChunkOverlapTokens
	}
	if file.SearchTopK > 0 {
		m.data.SearchTopK = file.SearchTopK
	}
	if file.SearchScoreThreshold >= 0 {
		m.data.SearchScoreThreshold = file.SearchScoreThreshold
	}
	m.data.SearchHybridEnabled = file.SearchHybridEnabled
	if file.SearchRecallK > 0 {
		m.data.SearchRecallK = file.SearchRecallK
	}
	m.data.SearchRerankEnabled = file.SearchRerankEnabled
	if file.RerankAPIURL != "" {
		m.data.RerankAPIURL = file.RerankAPIURL
	}
	if file.RerankModel != "" {
		m.data.RerankModel = file.RerankModel
	}

	if file.MilvusIndexType != "" {
		m.data.MilvusIndexType = file.MilvusIndexType
	}
	if file.MilvusMetric != "" {
		m.data.MilvusMetric = file.MilvusMetric
	}
	if file.MilvusNList > 0 {
		m.data.MilvusNList = file.MilvusNList
	}
	if file.MilvusNProbe > 0 {
		m.data.MilvusNProbe = file.MilvusNProbe
	}
	if file.MilvusHNSWM > 0 {
		m.data.MilvusHNSWM = file.MilvusHNSWM
	}
	if file.MilvusHNSWEfConstruction > 0 {
		m.data.MilvusHNSWEfConstruction = file.MilvusHNSWEfConstruction
	}
	if file.MilvusHNSWEf > 0 {
		m.data.MilvusHNSWEf = file.MilvusHNSWEf
	}
	if file.MilvusSparseDropRatioBuild >= 0 {
		m.data.MilvusSparseDropRatioBuild = file.MilvusSparseDropRatioBuild
	}
	if file.MilvusSparseDropRatioSearch >= 0 {
		m.data.MilvusSparseDropRatioSearch = file.MilvusSparseDropRatioSearch
	}

	if file.ChunkDedupScope != "" {
		m.data.ChunkDedupScope = file.ChunkDedupScope
	}
	if file.DocumentDedupMode != "" {
		m.data.DocumentDedupEnabled = file.DocumentDedupEnabled
		m.data.DocumentDedupByContentHash = file.DocumentDedupByContentHash
		m.data.DocumentDedupBySourceURI = file.DocumentDedupBySourceURI
		m.data.ChunkDedupEnabled = file.ChunkDedupEnabled
	} else {
		def := defaultDedupSettings()
		m.data.DocumentDedupEnabled = def.DocumentDedupEnabled
		m.data.DocumentDedupMode = def.DocumentDedupMode
		m.data.DocumentDedupByContentHash = def.DocumentDedupByContentHash
		m.data.DocumentDedupBySourceURI = def.DocumentDedupBySourceURI
		m.data.ChunkDedupEnabled = def.ChunkDedupEnabled
		m.data.ChunkDedupScope = def.ChunkDedupScope
	}
}

func (m *Manager) Get() AppSettings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data
}

func (m *Manager) PublicView(embeddingReady bool, embeddingStatus string, reindex ReindexView) PublicView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	view := PublicView{
		AppSettings:        m.data.withMilvusDefaults().withDedupDefaults(),
		SettingsPath:       m.path,
		EmbeddingAPIKeySet: m.data.EmbeddingAPIKey != "",
		LLMAPIKeySet:       m.data.LLMAPIKey != "",
		EmbeddingReady:     embeddingReady,
		EmbeddingStatus:    embeddingStatus,
		Reindex:            reindex,
	}
	view.EmbeddingAPIKey = ""
	view.LLMAPIKey = ""
	return view
}

// UpdateInput 部分更新；密钥留空或 ******** 表示不修改
type UpdateInput struct {
	MonitorURL *string `json:"monitor_url"`

	EmbeddingProvider     *string `json:"embedding_provider"`
	EmbeddingLocalBackend *string `json:"embedding_local_backend"`
	EmbeddingAPIURL       *string `json:"embedding_api_url"`
	EmbeddingAPIKey       *string `json:"embedding_api_key"`
	EmbeddingModel        *string `json:"embedding_model"`
	EmbeddingDim          *int    `json:"embedding_dim"`
	EmbeddingBatchSize    *int    `json:"embedding_batch_size"`
	EmbeddingMaxLength    *int    `json:"embedding_max_length"`

	LLMProvider     *string  `json:"llm_provider"`
	LLMLocalBackend *string  `json:"llm_local_backend"`
	LLMAPIURL       *string  `json:"llm_api_url"`
	LLMAPIKey       *string  `json:"llm_api_key"`
	LLMModel        *string  `json:"llm_model"`
	LLMTemperature  *float64 `json:"llm_temperature"`
	LLMMaxTokens    *int     `json:"llm_max_tokens"`

	ChunkMaxTokens     *int `json:"chunk_max_tokens"`
	ChunkOverlapTokens *int `json:"chunk_overlap_tokens"`

	SearchTopK           *int     `json:"search_top_k"`
	SearchScoreThreshold *float64 `json:"search_score_threshold"`
	SearchHybridEnabled  *bool    `json:"search_hybrid_enabled"`
	SearchRecallK        *int     `json:"search_recall_k"`
	SearchRerankEnabled  *bool    `json:"search_rerank_enabled"`
	RerankAPIURL         *string  `json:"rerank_api_url"`
	RerankModel          *string  `json:"rerank_model"`

	MilvusIndexType          *string `json:"milvus_index_type"`
	MilvusMetric             *string `json:"milvus_metric"`
	MilvusNList              *int    `json:"milvus_nlist"`
	MilvusNProbe             *int    `json:"milvus_nprobe"`
	MilvusHNSWM              *int    `json:"milvus_hnsw_m"`
	MilvusHNSWEfConstruction *int    `json:"milvus_hnsw_ef_construction"`
	MilvusHNSWEf             *int    `json:"milvus_hnsw_ef"`
	MilvusSparseDropRatioBuild  *float64 `json:"milvus_sparse_drop_ratio_build"`
	MilvusSparseDropRatioSearch *float64 `json:"milvus_sparse_drop_ratio_search"`

	DocumentDedupEnabled       *bool   `json:"document_dedup_enabled"`
	DocumentDedupMode          *string `json:"document_dedup_mode"`
	DocumentDedupByContentHash *bool   `json:"document_dedup_by_content_hash"`
	DocumentDedupBySourceURI   *bool   `json:"document_dedup_by_source_uri"`
	ChunkDedupEnabled          *bool   `json:"chunk_dedup_enabled"`
	ChunkDedupScope            *string `json:"chunk_dedup_scope"`
}

func (m *Manager) Update(in UpdateInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if in.MonitorURL != nil {
		m.data.MonitorURL = *in.MonitorURL
	}
	applyStrPtr(&m.data.EmbeddingProvider, in.EmbeddingProvider)
	applyStrPtr(&m.data.EmbeddingLocalBackend, in.EmbeddingLocalBackend)
	applyStrPtr(&m.data.EmbeddingAPIURL, in.EmbeddingAPIURL)
	if in.EmbeddingAPIKey != nil && *in.EmbeddingAPIKey != "" && *in.EmbeddingAPIKey != maskedSecret {
		m.data.EmbeddingAPIKey = *in.EmbeddingAPIKey
	}
	applyStrPtr(&m.data.EmbeddingModel, in.EmbeddingModel)
	if in.EmbeddingDim != nil && *in.EmbeddingDim > 0 {
		m.data.EmbeddingDim = *in.EmbeddingDim
	}
	if in.EmbeddingBatchSize != nil && *in.EmbeddingBatchSize > 0 {
		m.data.EmbeddingBatchSize = *in.EmbeddingBatchSize
	}
	if in.EmbeddingMaxLength != nil && *in.EmbeddingMaxLength > 0 {
		m.data.EmbeddingMaxLength = *in.EmbeddingMaxLength
	}

	applyStrPtr(&m.data.LLMProvider, in.LLMProvider)
	applyStrPtr(&m.data.LLMLocalBackend, in.LLMLocalBackend)
	applyStrPtr(&m.data.LLMAPIURL, in.LLMAPIURL)
	if in.LLMAPIKey != nil && *in.LLMAPIKey != "" && *in.LLMAPIKey != maskedSecret {
		m.data.LLMAPIKey = *in.LLMAPIKey
	}
	applyStrPtr(&m.data.LLMModel, in.LLMModel)
	if in.LLMTemperature != nil && *in.LLMTemperature >= 0 {
		m.data.LLMTemperature = *in.LLMTemperature
	}
	if in.LLMMaxTokens != nil && *in.LLMMaxTokens > 0 {
		m.data.LLMMaxTokens = *in.LLMMaxTokens
	}

	if in.ChunkMaxTokens != nil && *in.ChunkMaxTokens > 0 {
		m.data.ChunkMaxTokens = *in.ChunkMaxTokens
	}
	if in.ChunkOverlapTokens != nil && *in.ChunkOverlapTokens >= 0 {
		m.data.ChunkOverlapTokens = *in.ChunkOverlapTokens
	}
	if in.SearchTopK != nil && *in.SearchTopK > 0 {
		m.data.SearchTopK = *in.SearchTopK
	}
	if in.SearchScoreThreshold != nil && *in.SearchScoreThreshold >= 0 {
		m.data.SearchScoreThreshold = *in.SearchScoreThreshold
	}
	if in.SearchHybridEnabled != nil {
		m.data.SearchHybridEnabled = *in.SearchHybridEnabled
	}
	if in.SearchRecallK != nil && *in.SearchRecallK > 0 {
		m.data.SearchRecallK = *in.SearchRecallK
	}
	if in.SearchRerankEnabled != nil {
		m.data.SearchRerankEnabled = *in.SearchRerankEnabled
	}
	applyStrPtr(&m.data.RerankAPIURL, in.RerankAPIURL)
	applyStrPtr(&m.data.RerankModel, in.RerankModel)

	applyStrPtr(&m.data.MilvusIndexType, in.MilvusIndexType)
	applyStrPtr(&m.data.MilvusMetric, in.MilvusMetric)
	if in.MilvusNList != nil && *in.MilvusNList > 0 {
		m.data.MilvusNList = *in.MilvusNList
	}
	if in.MilvusNProbe != nil && *in.MilvusNProbe > 0 {
		m.data.MilvusNProbe = *in.MilvusNProbe
	}
	if in.MilvusHNSWM != nil && *in.MilvusHNSWM > 0 {
		m.data.MilvusHNSWM = *in.MilvusHNSWM
	}
	if in.MilvusHNSWEfConstruction != nil && *in.MilvusHNSWEfConstruction > 0 {
		m.data.MilvusHNSWEfConstruction = *in.MilvusHNSWEfConstruction
	}
	if in.MilvusHNSWEf != nil && *in.MilvusHNSWEf > 0 {
		m.data.MilvusHNSWEf = *in.MilvusHNSWEf
	}
	if in.MilvusSparseDropRatioBuild != nil && *in.MilvusSparseDropRatioBuild >= 0 {
		m.data.MilvusSparseDropRatioBuild = *in.MilvusSparseDropRatioBuild
	}
	if in.MilvusSparseDropRatioSearch != nil && *in.MilvusSparseDropRatioSearch >= 0 {
		m.data.MilvusSparseDropRatioSearch = *in.MilvusSparseDropRatioSearch
	}

	if in.DocumentDedupEnabled != nil {
		m.data.DocumentDedupEnabled = *in.DocumentDedupEnabled
	}
	applyStrPtr(&m.data.DocumentDedupMode, in.DocumentDedupMode)
	if in.DocumentDedupByContentHash != nil {
		m.data.DocumentDedupByContentHash = *in.DocumentDedupByContentHash
	}
	if in.DocumentDedupBySourceURI != nil {
		m.data.DocumentDedupBySourceURI = *in.DocumentDedupBySourceURI
	}
	if in.ChunkDedupEnabled != nil {
		m.data.ChunkDedupEnabled = *in.ChunkDedupEnabled
	}
	applyStrPtr(&m.data.ChunkDedupScope, in.ChunkDedupScope)

	if err := m.saveLocked(); err != nil {
		return err
	}
	m.applyToEnv(m.data)
	return nil
}

func applyStrPtr(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, raw, 0o600)
}

func (m *Manager) applyToEnv(s AppSettings) {
	setEnv("FLUXSEARCH_MONITOR_URL", s.MonitorURL)
	setEnv("FLUXSEARCH_EMBEDDING_PROVIDER", s.EmbeddingProvider)
	setEnv("FLUXSEARCH_EMBEDDING_LOCAL_BACKEND", s.EmbeddingLocalBackend)
	setEnv("FLUXSEARCH_EMBEDDING_API_URL", s.EmbeddingAPIURL)
	setEnv("FLUXSEARCH_EMBEDDING_API_KEY", s.EmbeddingAPIKey)
	setEnv("FLUXSEARCH_EMBEDDING_MODEL", s.EmbeddingModel)
	setEnv("FLUXSEARCH_EMBEDDING_DIM", strconv.Itoa(s.EmbeddingDim))
	setEnv("FLUXSEARCH_EMBEDDING_BATCH_SIZE", strconv.Itoa(s.EmbeddingBatchSize))
	setEnv("FLUXSEARCH_EMBEDDING_MAX_LENGTH", strconv.Itoa(s.EmbeddingMaxLength))

	setEnv("FLUXSEARCH_LLM_PROVIDER", s.LLMProvider)
	setEnv("FLUXSEARCH_LLM_LOCAL_BACKEND", s.LLMLocalBackend)
	setEnv("FLUXSEARCH_LLM_API_URL", s.LLMAPIURL)
	setEnv("FLUXSEARCH_LLM_API_KEY", s.LLMAPIKey)
	setEnv("FLUXSEARCH_LLM_MODEL", s.LLMModel)
	setEnv("FLUXSEARCH_LLM_TEMPERATURE", strconv.FormatFloat(s.LLMTemperature, 'f', -1, 64))
	setEnv("FLUXSEARCH_LLM_MAX_TOKENS", strconv.Itoa(s.LLMMaxTokens))

	setEnv("FLUXSEARCH_CHUNK_MAX_TOKENS", strconv.Itoa(s.ChunkMaxTokens))
	setEnv("FLUXSEARCH_CHUNK_OVERLAP_TOKENS", strconv.Itoa(s.ChunkOverlapTokens))
	setEnv("FLUXSEARCH_SEARCH_TOP_K", strconv.Itoa(s.SearchTopK))
	setEnv("FLUXSEARCH_SEARCH_SCORE_THRESHOLD", strconv.FormatFloat(s.SearchScoreThreshold, 'f', -1, 64))
	setEnv("FLUXSEARCH_SEARCH_HYBRID_ENABLED", strconv.FormatBool(s.SearchHybridEnabled))
	setEnv("FLUXSEARCH_SEARCH_RECALL_K", strconv.Itoa(s.SearchRecallK))
	setEnv("FLUXSEARCH_SEARCH_RERANK_ENABLED", strconv.FormatBool(s.SearchRerankEnabled))
	setEnv("FLUXSEARCH_RERANK_API_URL", s.RerankAPIURL)
	setEnv("FLUXSEARCH_RERANK_MODEL", s.RerankModel)

	setEnv("FLUXSEARCH_MILVUS_INDEX_TYPE", s.MilvusIndexType)
	setEnv("FLUXSEARCH_MILVUS_METRIC", s.MilvusMetric)
	setEnv("FLUXSEARCH_MILVUS_NLIST", strconv.Itoa(s.MilvusNList))
	setEnv("FLUXSEARCH_MILVUS_NPROBE", strconv.Itoa(s.MilvusNProbe))
	setEnv("FLUXSEARCH_MILVUS_HNSW_M", strconv.Itoa(s.MilvusHNSWM))
	setEnv("FLUXSEARCH_MILVUS_HNSW_EF_CONSTRUCTION", strconv.Itoa(s.MilvusHNSWEfConstruction))
	setEnv("FLUXSEARCH_MILVUS_HNSW_EF", strconv.Itoa(s.MilvusHNSWEf))
	setEnv("FLUXSEARCH_MILVUS_SPARSE_DROP_RATIO_BUILD", strconv.FormatFloat(s.MilvusSparseDropRatioBuild, 'f', -1, 64))
	setEnv("FLUXSEARCH_MILVUS_SPARSE_DROP_RATIO_SEARCH", strconv.FormatFloat(s.MilvusSparseDropRatioSearch, 'f', -1, 64))
}

func (m *Manager) ToConfig() config.Config {
	s := m.Get()
	cfg := config.Load()
	cfg.EmbeddingProvider = s.EmbeddingProvider
	cfg.EmbeddingLocalBackend = s.EmbeddingLocalBackend
	cfg.EmbeddingAPIURL = s.EmbeddingAPIURL
	cfg.EmbeddingAPIKey = s.EmbeddingAPIKey
	cfg.EmbeddingModel = s.EmbeddingModel
	cfg.EmbeddingDim = s.EmbeddingDim
	cfg.EmbeddingBatchSize = s.EmbeddingBatchSize
	cfg.EmbeddingMaxLength = s.EmbeddingMaxLength
	cfg.ChunkMaxTokens = s.ChunkMaxTokens
	cfg.ChunkOverlapTokens = s.ChunkOverlapTokens
	return cfg
}

func (m *Manager) MonitorURL() string {
	return m.Get().MonitorURL
}

func setEnv(key, val string) {
	_ = os.Setenv(key, val)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return n
}

func nonZeroInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
