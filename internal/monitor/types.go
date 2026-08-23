package monitor

import "github.com/fluxsearch/fluxsearch/internal/health"

type Metrics struct {
	DocumentsTotal    int64   `json:"documents_total"`
	ChunksTotal       int64   `json:"chunks_total"`
	CollectionsTotal  int     `json:"collections_total"`
	VectorEntities    int64   `json:"vector_entities"`
	MinioObjects      int64   `json:"minio_objects"`
	MinioBytes        int64   `json:"minio_bytes"`
	RedisKeys         int64   `json:"redis_keys"`
	RedisMemoryMB     float64 `json:"redis_memory_mb"`
	PostgresSizeMB    float64 `json:"postgres_size_mb"`
}

type Report struct {
	CheckedAt  string               `json:"checked_at"`
	Overall    health.Status        `json:"overall"`
	Source     string               `json:"source"`
	MonitorURL string               `json:"monitor_url,omitempty"`
	Host       string               `json:"host,omitempty"`
	Services   []health.ServiceCheck `json:"services"`
	Metrics    Metrics              `json:"metrics"`
}
