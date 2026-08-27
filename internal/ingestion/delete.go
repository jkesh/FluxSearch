package ingestion

import (
	"context"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
)

func (s *Service) persistOriginal(ctx context.Context, doc document.Document, filename string, raw []byte) error {
	if s.objects == nil || len(raw) == 0 {
		return nil
	}
	key, err := s.objects.PutDocument(ctx, doc.CollectionID, doc.ID, filename, raw, "")
	if err != nil {
		return err
	}
	meta := doc.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	meta["minio_bucket"] = s.objects.Bucket()
	meta["minio_object"] = key
	meta["original_size"] = len(raw)
	return s.pg.UpdateDocumentMetadata(ctx, doc.ID, meta)
}

func (s *Service) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	if s.pg == nil {
		return fmt.Errorf("postgres unavailable")
	}
	doc, err := s.pg.GetDocument(ctx, id)
	if err != nil {
		return err
	}

	if s.milvus != nil {
		coll, err := s.pg.GetCollectionByID(ctx, doc.CollectionID)
		if err == nil {
			if err := s.milvus.DeleteByDocument(ctx, coll.MilvusCollection, id.String()); err != nil {
				return fmt.Errorf("delete milvus vectors: %w", err)
			}
		}
	}

	if s.objects != nil && doc.Metadata != nil {
		if key, _ := doc.Metadata["minio_object"].(string); key != "" {
			_ = s.objects.Delete(ctx, key)
		}
	}

	if s.bm25 != nil {
		s.bm25.DeleteByDocument(id)
	}

	if err := s.pg.DeleteDocument(ctx, id); err != nil {
		return err
	}
	return nil
}
