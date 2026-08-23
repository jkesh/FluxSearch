package postgres

import (
	"context"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
)

func (s *Store) GetCollectionByID(ctx context.Context, id uuid.UUID) (document.Collection, error) {
	const q = `
		SELECT id, name, description, embedding_model, milvus_collection, created_at, updated_at
		FROM collections
		WHERE id = $1`

	var c document.Collection
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.Name, &c.Description, &c.EmbeddingModel, &c.MilvusCollection,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return document.Collection{}, fmt.Errorf("get collection: %w", err)
	}
	return c, nil
}

func (s *Store) GetCollectionByName(ctx context.Context, name string) (document.Collection, error) {
	const q = `
		SELECT id, name, description, embedding_model, milvus_collection, created_at, updated_at
		FROM collections
		WHERE name = $1`

	var c document.Collection
	err := s.pool.QueryRow(ctx, q, name).Scan(
		&c.ID, &c.Name, &c.Description, &c.EmbeddingModel, &c.MilvusCollection,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return document.Collection{}, fmt.Errorf("get collection by name: %w", err)
	}
	return c, nil
}
