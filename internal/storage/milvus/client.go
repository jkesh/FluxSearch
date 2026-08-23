package milvus

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

type Store struct {
	client client.Client
	dim    int
	idx    IndexConfig
}

func NewStore(ctx context.Context, cfg config.Config, idx IndexConfig) (*Store, error) {
	addr := fmt.Sprintf("%s:%d", cfg.MilvusHost, cfg.MilvusPort)

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c, err := client.NewClient(dialCtx, client.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}

	dim := cfg.EmbeddingDim
	if dim <= 0 {
		dim = config.DefaultEmbeddingDim
	}
	return &Store{client: c, dim: dim, idx: idx.Normalized()}, nil
}

func (s *Store) SetIndexConfig(idx IndexConfig) {
	s.idx = idx.Normalized()
}

func (s *Store) IndexConfig() IndexConfig {
	return s.idx.Normalized()
}

func (s *Store) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Store) Client() client.Client {
	return s.client
}

func (s *Store) VectorDim() int {
	return s.dim
}

func (s *Store) SetVectorDim(dim int) {
	if dim > 0 {
		s.dim = dim
	}
}
