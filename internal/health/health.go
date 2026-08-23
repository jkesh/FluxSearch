package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
)

type ServiceCheck struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Status    Status `json:"status"`
	Endpoint  string `json:"endpoint"`
	LatencyMS int64  `json:"latency_ms"`
	Message   string `json:"message"`
}

type Report struct {
	CheckedAt string         `json:"checked_at"`
	Overall   Status         `json:"overall"`
	Services  []ServiceCheck `json:"services"`
}

type Checker struct {
	cfg config.Config
}

func NewChecker(cfg config.Config) *Checker {
	return &Checker{cfg: cfg}
}

func (c *Checker) CheckAll(ctx context.Context) Report {
	checks := []func(context.Context) ServiceCheck{
		c.checkAPI,
		c.checkPostgres,
		c.checkRedis,
		c.checkMinIO,
		c.checkMilvus,
		c.checkEtcd,
	}

	services := make([]ServiceCheck, 0, len(checks))
	for _, fn := range checks {
		services = append(services, fn(ctx))
	}

	overall := StatusUp
	downCount := 0
	for _, s := range services {
		if s.Status == StatusDown {
			downCount++
		}
	}
	if downCount == len(services) {
		overall = StatusDown
	} else if downCount > 0 {
		overall = StatusDegraded
	}

	return Report{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Overall:   overall,
		Services:  services,
	}
}

func (c *Checker) checkAPI(ctx context.Context) ServiceCheck {
	start := time.Now()
	s := ServiceCheck{
		Name:     "api",
		Label:    "FluxSearch API",
		Category: "application",
		Endpoint: "self",
		Status:   StatusUp,
		Message:  "running",
	}
	s.LatencyMS = time.Since(start).Milliseconds()
	return s
}

func (c *Checker) checkPostgres(ctx context.Context) ServiceCheck {
	endpoint := fmt.Sprintf("%s:%d", c.cfg.PostgresHost, c.cfg.PostgresPort)
	s := ServiceCheck{
		Name:     "postgresql",
		Label:    "PostgreSQL",
		Category: "database",
		Endpoint: endpoint,
	}

	if c.cfg.PostgresPassword == "" {
		s.Status = StatusDegraded
		s.Message = "password not configured"
		return s
	}

	start := time.Now()
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?connect_timeout=3",
		c.cfg.PostgresUser, c.cfg.PostgresPassword, endpoint, c.cfg.PostgresDB,
	)
	conn, err := pgx.Connect(ctx, connStr)
	s.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}

	s.Status = StatusUp
	s.Message = "connected"
	return s
}

func (c *Checker) checkRedis(ctx context.Context) ServiceCheck {
	endpoint := fmt.Sprintf("%s:%d", c.cfg.RedisHost, c.cfg.RedisPort)
	s := ServiceCheck{
		Name:     "redis",
		Label:    "Redis",
		Category: "cache",
		Endpoint: endpoint,
	}

	opts := &redis.Options{
		Addr:        endpoint,
		DialTimeout: 3 * time.Second,
	}
	if c.cfg.RedisPassword != "" {
		opts.Password = c.cfg.RedisPassword
	}

	start := time.Now()
	client := redis.NewClient(opts)
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		s.LatencyMS = time.Since(start).Milliseconds()
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}

	s.LatencyMS = time.Since(start).Milliseconds()
	s.Status = StatusUp
	s.Message = "connected"
	return s
}

func (c *Checker) checkMinIO(ctx context.Context) ServiceCheck {
	s := ServiceCheck{
		Name:     "minio",
		Label:    "MinIO",
		Category: "storage",
		Endpoint: c.cfg.MinioEndpoint,
	}

	if c.cfg.MinioAccessKey == "" || c.cfg.MinioSecretKey == "" {
		s.Status = StatusDegraded
		s.Message = "credentials not configured"
		return s
	}

	host, secure, err := parseEndpoint(c.cfg.MinioEndpoint, c.cfg.MinioUseSSL)
	if err != nil {
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}

	start := time.Now()
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(c.cfg.MinioAccessKey, c.cfg.MinioSecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		s.LatencyMS = time.Since(start).Milliseconds()
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}

	_, err = client.ListBuckets(ctx)
	s.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		s.Status = StatusDown
		s.Message = err.Error()
		return s
	}

	s.Status = StatusUp
	s.Message = "connected"
	return s
}

func (c *Checker) checkMilvus(ctx context.Context) ServiceCheck {
	grpcEndpoint := fmt.Sprintf("%s:%d", c.cfg.MilvusHost, c.cfg.MilvusPort)
	s := ServiceCheck{
		Name:     "milvus",
		Label:    "Milvus",
		Category: "vector",
		Endpoint: grpcEndpoint,
	}

	start := time.Now()
	if !tcpReachable(ctx, c.cfg.MilvusHost, c.cfg.MilvusPort) {
		s.LatencyMS = time.Since(start).Milliseconds()
		s.Status = StatusDown
		s.Message = "gRPC port unreachable"
		return s
	}
	s.LatencyMS = time.Since(start).Milliseconds()

	healthURL := fmt.Sprintf("http://%s:9091/healthz", c.cfg.MilvusHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err == nil {
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				s.Status = StatusUp
				s.Message = "healthy"
				return s
			}
		}
	}

	s.Status = StatusUp
	s.Message = "gRPC port open"
	return s
}

func (c *Checker) checkEtcd(ctx context.Context) ServiceCheck {
	endpoint := fmt.Sprintf("%s:%d", c.cfg.EtcdHost, c.cfg.EtcdPort)
	s := ServiceCheck{
		Name:     "etcd",
		Label:    "etcd",
		Category: "metadata",
		Endpoint: endpoint,
	}

	start := time.Now()
	if !tcpReachable(ctx, c.cfg.EtcdHost, c.cfg.EtcdPort) {
		s.LatencyMS = time.Since(start).Milliseconds()
		s.Status = StatusDown
		s.Message = "connection refused"
		return s
	}

	s.LatencyMS = time.Since(start).Milliseconds()
	s.Status = StatusUp
	s.Message = "port reachable"
	return s
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

func parseEndpoint(endpoint string, useSSL bool) (host string, secure bool, err error) {
	secure = useSSL
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	if strings.HasPrefix(endpoint, "https://") {
		secure = true
	}
	if endpoint == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	return endpoint, secure, nil
}
