package ingestion

import (
	"context"

	"github.com/google/uuid"
)

// ObjectStore persists original uploaded files (e.g. MinIO).
type ObjectStore interface {
	Bucket() string
	PutDocument(ctx context.Context, collectionID, documentID uuid.UUID, filename string, data []byte, contentType string) (key string, err error)
	Delete(ctx context.Context, key string) error
}
