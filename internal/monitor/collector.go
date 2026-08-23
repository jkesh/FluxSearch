package monitor

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/fluxsearch/fluxsearch/internal/health"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

type Collector struct {
	cfg  config.Config
	host string
}

func NewCollector(cfg config.Config) *Collector {
	host, _ := os.Hostname()
	return &Collector{cfg: cfg, host: host}
}

func (c *Collector) Collect(ctx context.Context) Report {
	checker := health.NewChecker(c.cfg)
	base := checker.CheckAll(ctx)

	metrics := Metrics{}
	c.collectPostgresMetrics(ctx, &metrics)
	c.collectRedisMetrics(ctx, &metrics)
	c.collectMinioMetrics(ctx, &metrics)
	c.collectMilvusMetrics(ctx, &metrics)

	return Report{
		CheckedAt: base.CheckedAt,
		Overall:   base.Overall,
		Source:    "monitor",
		Host:      c.host,
		Services:  base.Services,
		Metrics:   metrics,
	}
}

func (c *Collector) collectPostgresMetrics(ctx context.Context, m *Metrics) {
	if c.cfg.PostgresPassword == "" {
		return
	}
	endpoint := fmt.Sprintf("%s:%d", c.cfg.PostgresHost, c.cfg.PostgresPort)
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?connect_timeout=3",
		c.cfg.PostgresUser, c.cfg.PostgresPassword, endpoint, c.cfg.PostgresDB,
	)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return
	}
	defer conn.Close(ctx)

	// documents 表（V1 后创建）
	var exists bool
	_ = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'documents'
		)`).Scan(&exists)
	if exists {
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM documents`).Scan(&m.DocumentsTotal)
	}

	// chunks 表
	exists = false
	_ = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'chunks'
		)`).Scan(&exists)
	if exists {
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&m.ChunksTotal)
	}

	var sizeBytes int64
	_ = conn.QueryRow(ctx, `
		SELECT COALESCE(SUM(pg_total_relation_size(oid)), 0)::bigint
		FROM pg_class WHERE relkind = 'r' AND relnamespace = 'public'::regnamespace
	`).Scan(&sizeBytes)
	m.PostgresSizeMB = float64(sizeBytes) / 1024 / 1024
}

func (c *Collector) collectRedisMetrics(ctx context.Context, m *Metrics) {
	endpoint := fmt.Sprintf("%s:%d", c.cfg.RedisHost, c.cfg.RedisPort)
	opts := &redis.Options{Addr: endpoint, DialTimeout: 3 * time.Second}
	if c.cfg.RedisPassword != "" {
		opts.Password = c.cfg.RedisPassword
	}
	client := redis.NewClient(opts)
	defer client.Close()

	if n, err := client.DBSize(ctx).Result(); err == nil {
		m.RedisKeys = n
	}
	if info, err := client.Info(ctx, "memory").Result(); err == nil {
		m.RedisMemoryMB = parseRedisInfoFloat(info, "used_memory:")
	}
}

func (c *Collector) collectMinioMetrics(ctx context.Context, m *Metrics) {
	if c.cfg.MinioAccessKey == "" {
		return
	}
	host, secure, err := parseEndpoint(c.cfg.MinioEndpoint, c.cfg.MinioUseSSL)
	if err != nil {
		return
	}
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(c.cfg.MinioAccessKey, c.cfg.MinioSecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return
	}

	bucket := envOr("FLUXSEARCH_MINIO_BUCKET", "milvus-bucket")
	var count, totalBytes int64
	for obj := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			break
		}
		count++
		totalBytes += obj.Size
		if count >= 10000 {
			break
		}
	}
	m.MinioObjects = count
	m.MinioBytes = totalBytes
}

func (c *Collector) collectMilvusMetrics(ctx context.Context, m *Metrics) {
	if !tcpReachable(ctx, c.cfg.MilvusHost, c.cfg.MilvusPort) {
		return
	}
	// 通过 Milvus HTTP metrics 获取基础信息；集合详情待 SDK 接入
	m.CollectionsTotal = 0
	m.VectorEntities = 0
}

func tcpReachable(ctx context.Context, host string, port int) bool {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func parseEndpoint(endpoint string, useSSL bool) (string, bool, error) {
	secure := useSSL
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	if endpoint == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	return endpoint, secure, nil
}

func parseRedisInfoFloat(info, key string) float64 {
	for _, line := range splitLines(info) {
		if len(line) > len(key) && line[:len(key)] == key {
			var v float64
			fmt.Sscanf(line[len(key):], "%f", &v)
			return v / 1024 / 1024
		}
	}
	return 0
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
