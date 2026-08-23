package conversation

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusActive   = "active"
	StatusArchived = "archived"

	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"

	MaxHistoryMessages = 20
)

type Source struct {
	ChunkID    uuid.UUID `json:"chunk_id"`
	DocumentID uuid.UUID `json:"document_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	Score      float32   `json:"score"`
	Page       *int      `json:"page,omitempty"`
}

type Conversation struct {
	ID           uuid.UUID      `json:"id"`
	CollectionID uuid.UUID      `json:"collection_id"`
	Title        string         `json:"title"`
	Status       string         `json:"status"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type ListItem struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	MessageCount int       `json:"message_count"`
	LastPreview  string    `json:"last_preview"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Message struct {
	ID             uuid.UUID      `json:"id"`
	ConversationID uuid.UUID      `json:"conversation_id"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	Sources        []Source       `json:"sources,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type ChatRequest struct {
	Content        string
	ConversationID uuid.UUID
	CollectionID   uuid.UUID
}
