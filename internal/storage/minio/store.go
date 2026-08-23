package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/google/uuid"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultDocumentsBucket = "fluxsearch-documents"

type Store struct {
	client *minio.Client
	bucket string
}

func NewStore(ctx context.Context, cfg config.Config) (*Store, error) {
	if cfg.MinioAccessKey == "" || cfg.MinioSecretKey == "" {
		return nil, fmt.Errorf("minio credentials not configured")
	}
	host, secure, err := parseEndpoint(cfg.MinioEndpoint, cfg.MinioUseSSL)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	bucket := cfg.MinioDocumentsBucket
	if bucket == "" {
		bucket = defaultDocumentsBucket
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}
	return &Store{client: client, bucket: bucket}, nil
}

func (s *Store) Bucket() string { return s.bucket }

func (s *Store) PutImportStaging(ctx context.Context, jobID uuid.UUID, index int, filename string, data []byte) (string, error) {
	key := fmt.Sprintf("imports/pending/%s/%d_%s", jobID, index, sanitizeFilename(filename))
	return key, s.put(ctx, key, data, "application/octet-stream")
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get: %w", err)
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *Store) PutDocument(ctx context.Context, collectionID, documentID uuid.UUID, filename string, data []byte, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := fmt.Sprintf("documents/%s/%s/%s", collectionID, documentID, sanitizeFilename(filename))
	return key, s.put(ctx, key, data, contentType)
}

func (s *Store) put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio put %s: %w", key, err)
	}
	return nil
}

func parseEndpoint(endpoint string, useSSL bool) (host string, secure bool, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, fmt.Errorf("empty minio endpoint")
	}
	secure = useSSL
	if strings.HasPrefix(endpoint, "https://") {
		secure = true
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		secure = false
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	return strings.TrimRight(endpoint, "/"), secure, nil
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	return name
}
