package document

import (
	"time"

	"github.com/google/uuid"
)

const (
	DefaultCollectionID = "00000000-0000-0000-0000-000000000001"

	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusIndexed    = "indexed"
	StatusFailed     = "failed"

	ChunkActive  = "active"
	ChunkStale   = "stale"
	ChunkDeleted = "deleted"
)

type PageSnapshot struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

type Collection struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	EmbeddingModel   string    `json:"embedding_model"`
	MilvusCollection string    `json:"milvus_collection"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Document struct {
	ID           uuid.UUID      `json:"id"`
	CollectionID uuid.UUID      `json:"collection_id"`
	Title        string         `json:"title"`
	SourceType   string         `json:"source_type"`
	SourceURI    string         `json:"source_uri,omitempty"`
	ContentHash  string         `json:"content_hash,omitempty"`
	Content      string         `json:"content,omitempty"`
	ContentPages []PageSnapshot `json:"content_pages,omitempty"`
	Version      int            `json:"version"`
	Status       string         `json:"status"`
	ErrorMessage string         `json:"error_message,omitempty"`
	ChunkCount   int            `json:"chunk_count"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	IndexedAt    *time.Time     `json:"indexed_at,omitempty"`
}

// DocumentListItem 列表项（含摘要，不含全文）
type DocumentListItem struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	SourceType     string    `json:"source_type"`
	SourceURI      string    `json:"source_uri,omitempty"`
	Status         string    `json:"status"`
	ChunkCount     int       `json:"chunk_count"`
	Version        int       `json:"version"`
	ContentPreview string    `json:"content_preview"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Chunk struct {
	ID                    uuid.UUID      `json:"id"`
	DocumentID            uuid.UUID      `json:"document_id"`
	DocumentVersion       int            `json:"document_version"`
	ChunkIndex            int            `json:"chunk_index"`
	ChunkHash             string         `json:"chunk_hash"`
	Content               string         `json:"content"`
	TokenCount            int            `json:"token_count"`
	Page                  *int           `json:"page,omitempty"`
	Section               string         `json:"section,omitempty"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	EmbeddingModelVersion string         `json:"embedding_model_version,omitempty"`
	Status                string         `json:"status"`
	CreatedAt             time.Time      `json:"created_at"`
}

type SearchHit struct {
	ChunkID    uuid.UUID `json:"chunk_id"`
	DocumentID uuid.UUID `json:"document_id"`
	Content    string    `json:"content"`
	Score      float32   `json:"score"`
	Page       *int      `json:"page,omitempty"`
	Section    string    `json:"section,omitempty"`
}

type UpdateDocumentContentInput struct {
	Title        string
	SourceType   string
	ContentHash  string
	Content      string
	ContentPages []PageSnapshot
}

type CreateDocumentInput struct {
	CollectionID uuid.UUID
	Title        string
	SourceType   string
	SourceURI    string
	ContentHash  string
	Content      string
	ContentPages []PageSnapshot
	Metadata     map[string]any
}

type CreateChunkInput struct {
	DocumentID            uuid.UUID
	DocumentVersion       int
	ChunkIndex            int
	ChunkHash             string
	Content               string
	TokenCount            int
	Page                  *int
	Section               string
	Metadata              map[string]any
	EmbeddingModelVersion string
}

func PreviewText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 200
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}
