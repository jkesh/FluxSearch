package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const RedisChannel = "fluxsearch:events"

const (
	TypeDocumentCreated = "document.created"
	TypeDocumentUpdated = "document.updated"
	TypeDocumentDeleted = "document.deleted"
	TypeDocumentReindex = "document.reindex"
	TypeImportProgress  = "import_progress"
)

type Event struct {
	Type         string    `json:"type"`
	DocumentID   string    `json:"document_id,omitempty"`
	CollectionID string    `json:"collection_id,omitempty"`
	Title        string    `json:"title,omitempty"`
	Status       string    `json:"status,omitempty"`
	JobID        string    `json:"job_id,omitempty"`
	Message      string    `json:"message,omitempty"`
	Payload      any       `json:"payload,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func New(typ string) Event {
	return Event{Type: typ, Timestamp: time.Now().UTC()}
}

func DocumentCreated(docID, collectionID, title string) Event {
	e := New(TypeDocumentCreated)
	e.DocumentID = docID
	e.CollectionID = collectionID
	e.Title = title
	e.Status = "indexed"
	return e
}

func DocumentUpdated(docID, collectionID, title string) Event {
	e := New(TypeDocumentUpdated)
	e.DocumentID = docID
	e.CollectionID = collectionID
	e.Title = title
	e.Status = "indexed"
	return e
}

func DocumentDeleted(docID string) Event {
	e := New(TypeDocumentDeleted)
	e.DocumentID = docID
	return e
}

func DocumentReindex(docID, status, message string) Event {
	e := New(TypeDocumentReindex)
	e.DocumentID = docID
	e.Status = status
	e.Message = message
	return e
}

// Bus publishes domain events (Redis Pub/Sub; Kafka-compatible schema for future migration).
type Bus interface {
	Publish(ctx context.Context, ev Event) error
}

type noopBus struct{}

func (noopBus) Publish(context.Context, Event) error { return nil }

func Noop() Bus { return noopBus{} }

type Publisher interface {
	Bus
}

// Parse decodes a Redis message payload.
func Parse(data []byte) (Event, error) {
	var ev Event
	err := json.Unmarshal(data, &ev)
	return ev, err
}

func (e Event) Marshal() ([]byte, error) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	return json.Marshal(e)
}

// Helper for handlers that only have UUIDs.
func IDString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
