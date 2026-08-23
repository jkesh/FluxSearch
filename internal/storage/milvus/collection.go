package milvus

import (
	"context"
	"fmt"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	FieldChunkID               = "chunk_id"
	FieldDocumentID            = "document_id"
	FieldDocumentVersion       = "document_version"
	FieldContent               = "content"
	FieldDenseVector           = "dense_vector"
	FieldPage                  = "page"
	FieldSection               = "section"
	FieldEmbeddingModelVersion = "embedding_model_version"
)

func (s *Store) EnsureCollection(ctx context.Context, name string) error {
	exists, err := s.client.HasCollection(ctx, name)
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}
	if exists {
		return s.client.LoadCollection(ctx, name, false)
	}
	return s.createCollection(ctx, name)
}

func (s *Store) RecreateCollection(ctx context.Context, name string) error {
	if err := s.DropCollection(ctx, name); err != nil {
		return err
	}
	return s.createCollection(ctx, name)
}

func (s *Store) createCollection(ctx context.Context, name string) error {
	idx := s.idx.Normalized()
	schema := &entity.Schema{
		CollectionName: name,
		Description:    "FluxSearch chunk vectors",
		AutoID:         false,
		Fields: []*entity.Field{
			{
				Name:       FieldChunkID,
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{"max_length": "36"},
			},
			{
				Name:     FieldDocumentID,
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "36"},
			},
			{
				Name:     FieldDocumentVersion,
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     FieldContent,
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "8192"},
			},
			{
				Name:     FieldDenseVector,
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					entity.TypeParamDim: fmt.Sprintf("%d", s.dim),
				},
			},
			{
				Name:     FieldPage,
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:     FieldSection,
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "512"},
			},
			{
				Name:     FieldEmbeddingModelVersion,
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": "128"},
			},
		},
	}

	if err := s.client.CreateCollection(ctx, schema, 1); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	index, err := s.buildIndex(idx)
	if err != nil {
		return fmt.Errorf("new index: %w", err)
	}
	if err := s.client.CreateIndex(ctx, name, FieldDenseVector, index, false); err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	return s.client.LoadCollection(ctx, name, false)
}

func (s *Store) buildIndex(idx IndexConfig) (entity.Index, error) {
	metric := idx.MetricType()
	switch strings.ToLower(idx.IndexType) {
	case IndexTypeHNSW:
		return entity.NewIndexHNSW(metric, idx.HNSWM, idx.HNSWEfConstruction)
	default:
		return entity.NewIndexIvfFlat(metric, idx.NList)
	}
}

func (s *Store) DropCollection(ctx context.Context, name string) error {
	exists, err := s.client.HasCollection(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return s.client.DropCollection(ctx, name)
}

func (s *Store) Stats(ctx context.Context, name string) (int64, error) {
	stats, err := s.client.GetCollectionStatistics(ctx, name)
	if err != nil {
		return 0, err
	}
	if v, ok := stats["row_count"]; ok {
		var n int64
		_, _ = fmt.Sscan(v, &n)
		return n, nil
	}
	return 0, nil
}
