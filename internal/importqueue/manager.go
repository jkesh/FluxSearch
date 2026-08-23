package importqueue

import (
	"context"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/ingestion"
	"github.com/google/uuid"
)

const (
	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusPartial   = "partial"
	JobStatusFailed    = "failed"

	ItemStatusQueued     = "queued"
	ItemStatusProcessing = "processing"
	ItemStatusDone       = "done"
	ItemStatusSkipped    = "skipped"
	ItemStatusUpdated    = "updated"
	ItemStatusFailed     = "failed"
)

type FileInput struct {
	Filename   string
	SourceType string
	Raw        []byte
	StagingKey string
}

type Item struct {
	Filename      string     `json:"filename"`
	Status        string     `json:"status"`
	Outcome       string     `json:"outcome,omitempty"`
	DocumentID    *uuid.UUID `json:"document_id,omitempty"`
	Title         string     `json:"title,omitempty"`
	ChunkCount    int        `json:"chunk_count,omitempty"`
	VectorsStored bool       `json:"vectors_stored,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type Job struct {
	ID           uuid.UUID  `json:"id"`
	CollectionID uuid.UUID  `json:"collection_id"`
	Status       string     `json:"status"`
	Total        int        `json:"total"`
	Done         int        `json:"done"`
	Failed       int        `json:"failed"`
	Progress     int        `json:"progress"`
	Items        []Item     `json:"items"`
	Message      string     `json:"message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type Runner interface {
	ImportFile(ctx context.Context, collectionID uuid.UUID, in FileInput) (ingestion.ImportResult, error)
}

func percent(done, total int) int {
	if total <= 0 {
		return 0
	}
	return done * 100 / total
}
