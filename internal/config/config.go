package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	RedisHost     string
	RedisPort     int
	RedisPassword string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioUseSSL    bool
	MinioDocumentsBucket string

	MilvusHost string
	MilvusPort int

	EtcdHost string
	EtcdPort int

	EmbeddingDim int

	EmbeddingProvider     string
	EmbeddingAPIURL       string
	EmbeddingAPIKey       string
	EmbeddingModel        string
	EmbeddingBatchSize    int
	EmbeddingMaxLength    int
	EmbeddingLocalBackend string

	ChunkMaxTokens    int
	ChunkOverlapTokens int

	// When false, API does not run import/reindex worker (use cmd/worker).
	ImportWorkerInAPI bool
}

const (
	DefaultEmbeddingDim   = 1024
	DefaultChunkMaxTokens     = 512
	DefaultChunkOverlapTokens = 64
	DefaultEmbeddingBatch       = 16
	DefaultEmbeddingMaxLength = 512
)

func Load() Config {
	loadEnvFiles()

	pgHost := envOr("FLUXSEARCH_POSTGRES_HOST_LOCAL", envOr("FLUXSEARCH_POSTGRES_HOST", "127.0.0.1"))
	redisHost := envOr("FLUXSEARCH_REDIS_HOST_LOCAL", envOr("FLUXSEARCH_REDIS_HOST", "127.0.0.1"))
	minioEndpoint := envOr("FLUXSEARCH_MINIO_ENDPOINT_LOCAL", envOr("FLUXSEARCH_MINIO_ENDPOINT", "127.0.0.1:9000"))
	milvusHost := envOr("FLUXSEARCH_MILVUS_HOST_LOCAL", envOr("FLUXSEARCH_MILVUS_HOST", "127.0.0.1"))
	etcdHost := envOr("FLUXSEARCH_ETCD_HOST_LOCAL", envOr("FLUXSEARCH_ETCD_HOST", "127.0.0.1"))

	// 配置了 LOCAL 地址时，其余服务默认也走本地 port-forward
	if local := envOr("FLUXSEARCH_POSTGRES_HOST_LOCAL", ""); local != "" {
		if envOr("FLUXSEARCH_REDIS_HOST_LOCAL", "") == "" {
			redisHost = "127.0.0.1"
		}
		if envOr("FLUXSEARCH_ETCD_HOST_LOCAL", "") == "" {
			etcdHost = "127.0.0.1"
		}
		if envOr("FLUXSEARCH_MINIO_ENDPOINT_LOCAL", "") == "" {
			minioEndpoint = "127.0.0.1:9000"
		}
		if envOr("FLUXSEARCH_MILVUS_HOST_LOCAL", "") == "" {
			milvusHost = "127.0.0.1"
		}
	}

	return Config{
		PostgresHost:     pgHost,
		PostgresPort:     envInt("FLUXSEARCH_POSTGRES_PORT_LOCAL", envInt("FLUXSEARCH_POSTGRES_PORT", 5432)),
		PostgresUser:     envOr("FLUXSEARCH_POSTGRES_USER", envOr("POSTGRES_USER", "fluxsearch")),
		PostgresPassword: envOr("FLUXSEARCH_POSTGRES_PASSWORD", envOr("POSTGRES_PASSWORD", "")),
		PostgresDB:       envOr("FLUXSEARCH_POSTGRES_DB", envOr("POSTGRES_DB", "fluxsearch")),

		RedisHost:     redisHost,
		RedisPort:     envInt("FLUXSEARCH_REDIS_PORT_LOCAL", envInt("FLUXSEARCH_REDIS_PORT", 6379)),
		RedisPassword: envOr("FLUXSEARCH_REDIS_PASSWORD", envOr("REDIS_PASSWORD", "")),

		MinioEndpoint:  minioEndpoint,
		MinioAccessKey: envOr("FLUXSEARCH_MINIO_ACCESS_KEY", envOr("MINIO_ACCESS_KEY", "")),
		MinioSecretKey: envOr("FLUXSEARCH_MINIO_SECRET_KEY", envOr("MINIO_SECRET_KEY", "")),
		MinioUseSSL:    envOr("FLUXSEARCH_MINIO_USE_SSL", "false") == "true",
		MinioDocumentsBucket: envOr("FLUXSEARCH_MINIO_DOCUMENTS_BUCKET", "fluxsearch-documents"),

		MilvusHost: milvusHost,
		MilvusPort: envInt("FLUXSEARCH_MILVUS_PORT_LOCAL", envInt("FLUXSEARCH_MILVUS_PORT", 19530)),

		EtcdHost: etcdHost,
		EtcdPort: envInt("FLUXSEARCH_ETCD_PORT_LOCAL", envInt("FLUXSEARCH_ETCD_PORT", 2379)),

		EmbeddingDim: EmbeddingDim(),

		EmbeddingProvider:     envOr("FLUXSEARCH_EMBEDDING_PROVIDER", ""),
		EmbeddingAPIURL:       envOr("FLUXSEARCH_EMBEDDING_API_URL", ""),
		EmbeddingAPIKey:       envOr("FLUXSEARCH_EMBEDDING_API_KEY", ""),
		EmbeddingModel:        envOr("FLUXSEARCH_EMBEDDING_MODEL", ""),
		EmbeddingBatchSize:    envInt("FLUXSEARCH_EMBEDDING_BATCH_SIZE", DefaultEmbeddingBatch),
		EmbeddingMaxLength:    envInt("FLUXSEARCH_EMBEDDING_MAX_LENGTH", DefaultEmbeddingMaxLength),
		EmbeddingLocalBackend: envOr("FLUXSEARCH_EMBEDDING_LOCAL_BACKEND", "ollama"),

		ChunkMaxTokens:     chunkMaxTokensFromEnv(),
		ChunkOverlapTokens: chunkOverlapTokensFromEnv(),

		ImportWorkerInAPI: envOr("FLUXSEARCH_IMPORT_WORKER_IN_API", "true") != "false",
	}
}

// ImportWorkerInAPI returns whether the API process should consume the import queue.
func ImportWorkerInAPI() bool {
	return envOr("FLUXSEARCH_IMPORT_WORKER_IN_API", "true") != "false"
}

func EmbeddingDim() int {
	return envInt("FLUXSEARCH_EMBEDDING_DIM", DefaultEmbeddingDim)
}

// CharsToTokens 将旧版字符数配置换算为 token 数（4 字符 ≈ 1 token）
func CharsToTokens(chars int) int {
	return charsToTokens(chars)
}

func charsToTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	tokens := chars / 4
	if tokens < 1 {
		return 1
	}
	return tokens
}

func chunkMaxTokensFromEnv() int {
	if v := os.Getenv("FLUXSEARCH_CHUNK_MAX_TOKENS"); v != "" {
		return envInt("FLUXSEARCH_CHUNK_MAX_TOKENS", DefaultChunkMaxTokens)
	}
	if v := os.Getenv("FLUXSEARCH_CHUNK_MAX_CHARS"); v != "" {
		return charsToTokens(envInt("FLUXSEARCH_CHUNK_MAX_CHARS", 0))
	}
	return DefaultChunkMaxTokens
}

func chunkOverlapTokensFromEnv() int {
	if v := os.Getenv("FLUXSEARCH_CHUNK_OVERLAP_TOKENS"); v != "" {
		return envInt("FLUXSEARCH_CHUNK_OVERLAP_TOKENS", DefaultChunkOverlapTokens)
	}
	if v := os.Getenv("FLUXSEARCH_CHUNK_OVERLAP"); v != "" {
		return charsToTokens(envInt("FLUXSEARCH_CHUNK_OVERLAP", 0))
	}
	return DefaultChunkOverlapTokens
}

func loadEnvFiles() {
	infraCandidates := []string{
		"config/local/infra.env",
		"../config/local/infra.env",
		filepath.Join("..", "..", "config", "local", "infra.env"),
	}
	for _, path := range infraCandidates {
		if err := loadEnvFile(path); err == nil {
			break
		}
	}
	deployCandidates := []string{
		"config/local/deploy.env",
		"../config/local/deploy.env",
		filepath.Join("..", "..", "config", "local", "deploy.env"),
	}
	for _, path := range deployCandidates {
		if err := loadEnvFile(path); err == nil {
			break
		}
	}
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
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
