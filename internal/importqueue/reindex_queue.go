package importqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/events"
	"github.com/fluxsearch/fluxsearch/internal/ingestion"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const redisKeyReindexQueue = "fluxsearch:reindex:queue"

type ReindexRunner interface {
	ReimportFile(ctx context.Context, documentID uuid.UUID, in FileInput) (ingestion.ImportResult, error)
	RechunkDocument(ctx context.Context, documentID uuid.UUID) error
}

type reindexPayload struct {
	DocumentID uuid.UUID   `json:"document_id"`
	File       *storedFile `json:"file,omitempty"`
}

// EnqueueReimport stages a new file and queues async reindex for an existing document.
func (m *Manager) EnqueueReimport(ctx context.Context, documentID uuid.UUID, in FileInput) error {
	if m.rdb == nil || m.minio == nil {
		return fmt.Errorf("redis/minio unavailable")
	}
	jobID := uuid.New()
	key, err := m.minio.PutImportStaging(ctx, jobID, 0, in.Filename, in.Raw)
	if err != nil {
		return fmt.Errorf("stage reimport: %w", err)
	}
	payload := reindexPayload{
		DocumentID: documentID,
		File: &storedFile{
			Filename:   in.Filename,
			SourceType: in.SourceType,
			ObjectKey:  key,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := m.rdb.LPush(ctx, redisKeyReindexQueue, string(raw)).Err(); err != nil {
		return err
	}
	return m.publish(ctx, events.DocumentReindex(documentID.String(), "queued", "reimport queued"))
}

// EnqueueRechunk queues async rechunk for a document (content already in PG).
func (m *Manager) EnqueueRechunk(ctx context.Context, documentID uuid.UUID) error {
	if m.rdb == nil {
		return fmt.Errorf("redis unavailable")
	}
	payload := reindexPayload{DocumentID: documentID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := m.rdb.LPush(ctx, redisKeyReindexQueue, string(raw)).Err(); err != nil {
		return err
	}
	return m.publish(ctx, events.DocumentReindex(documentID.String(), "queued", "rechunk queued"))
}

func (m *Manager) publish(ctx context.Context, ev events.Event) error {
	if m.eventBus == nil {
		return nil
	}
	return m.eventBus.Publish(ctx, ev)
}

func (m *Manager) processReindex(ctx context.Context, runner ReindexRunner, raw string) {
	var payload reindexPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		log.Printf("reindex payload: %v", err)
		return
	}
	docID := payload.DocumentID
	_ = m.publish(ctx, events.DocumentReindex(docID.String(), "processing", ""))

	var err error
	var result ingestion.ImportResult
	if payload.File != nil {
		fileRaw, getErr := m.minio.Get(ctx, payload.File.ObjectKey)
		if getErr != nil {
			err = getErr
		} else {
			taskCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			result, err = runner.ReimportFile(taskCtx, docID, FileInput{
				Filename:   payload.File.Filename,
				SourceType: payload.File.SourceType,
				Raw:        fileRaw,
			})
			cancel()
		}
		_ = m.minio.Delete(ctx, payload.File.ObjectKey)
	} else {
		taskCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		err = runner.RechunkDocument(taskCtx, docID)
		cancel()
	}

	if err != nil {
		log.Printf("reindex %s: %v", docID, err)
		_ = m.publish(ctx, events.DocumentReindex(docID.String(), "failed", err.Error()))
		return
	}
	if payload.File != nil {
		_ = m.publish(ctx, events.DocumentUpdated(docID.String(), result.Document.CollectionID.String(), result.Document.Title))
	} else {
		_ = m.publish(ctx, events.DocumentUpdated(docID.String(), "", ""))
	}
}

func (m *Manager) workerLoopMulti(ctx context.Context, runner Runner, reindex ReindexRunner) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		res, err := m.rdb.BRPop(ctx, 5*time.Second, redisKeyQueue, redisKeyReindexQueue).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker brpop: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(res) < 2 {
			continue
		}
		switch res[0] {
		case redisKeyQueue:
			jobID, err := uuid.Parse(res[1])
			if err != nil {
				continue
			}
			m.processJob(ctx, runner, jobID)
		case redisKeyReindexQueue:
			m.processReindex(ctx, reindex, res[1])
		}
	}
}
