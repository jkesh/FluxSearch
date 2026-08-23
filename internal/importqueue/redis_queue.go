package importqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/ingestion"
	miniostore "github.com/fluxsearch/fluxsearch/internal/storage/minio"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	redisKeyQueue   = "fluxsearch:import:queue"
	redisKeyJobs    = "fluxsearch:import:jobs"
	redisKeyJobPref = "fluxsearch:import:job:"
	jobTTL          = 7 * 24 * time.Hour
)

type Broadcaster interface {
	BroadcastImport(job Job)
}

// NotifyFunc adapts a function to Broadcaster (e.g. WebSocket push from router).
type NotifyFunc func(job Job)

func (f NotifyFunc) BroadcastImport(job Job) {
	if f != nil {
		f(job)
	}
}

type storedFile struct {
	Filename   string `json:"filename"`
	SourceType string `json:"source_type"`
	ObjectKey  string `json:"object_key"`
}

type Manager struct {
	mu           sync.Mutex
	rdb          *redis.Client
	minio        *miniostore.Store
	broadcast    Broadcaster
	maxJobs      int
	workerCancel context.CancelFunc
}

func NewManager(rdb *redis.Client, minio *miniostore.Store) *Manager {
	return &Manager{
		rdb:     rdb,
		minio:   minio,
		maxJobs: 100,
	}
}

func (m *Manager) SetBroadcaster(b Broadcaster) {
	m.broadcast = b
}

func (m *Manager) StartWorker(ctx context.Context, runner Runner) {
	if m.rdb == nil {
		log.Printf("import worker: redis unavailable, worker not started")
		return
	}
	m.mu.Lock()
	if m.workerCancel != nil {
		m.workerCancel()
	}
	workerCtx, cancel := context.WithCancel(ctx)
	m.workerCancel = cancel
	m.mu.Unlock()

	go m.workerLoop(workerCtx, runner)
	log.Printf("import worker started (redis queue)")
}

func (m *Manager) Enqueue(ctx context.Context, collectionID uuid.UUID, files []FileInput) (*Job, error) {
	if m.rdb == nil {
		return nil, fmt.Errorf("redis unavailable")
	}
	if m.minio == nil {
		return nil, fmt.Errorf("minio unavailable")
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files")
	}

	jobID := uuid.New()
	now := time.Now().UTC()
	stored := make([]storedFile, 0, len(files))
	items := make([]Item, len(files))

	for i, f := range files {
		key, err := m.minio.PutImportStaging(ctx, jobID, i, f.Filename, f.Raw)
		if err != nil {
			return nil, fmt.Errorf("stage %s: %w", f.Filename, err)
		}
		stored = append(stored, storedFile{
			Filename:   f.Filename,
			SourceType: f.SourceType,
			ObjectKey:  key,
		})
		items[i] = Item{Filename: f.Filename, Status: ItemStatusQueued}
	}

	job := Job{
		ID:           jobID,
		CollectionID: collectionID,
		Status:       JobStatusQueued,
		Total:        len(files),
		Items:        items,
		CreatedAt:    now,
		UpdatedAt:    now,
		Message:      "已加入队列",
	}

	payload := redisJobPayload{
		Job:   job,
		Files: stored,
	}
	if err := m.saveJob(ctx, payload); err != nil {
		return nil, err
	}
	if err := m.rdb.LPush(ctx, redisKeyQueue, jobID.String()).Err(); err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}
	if err := m.rdb.LPush(ctx, redisKeyJobs, jobID.String()).Err(); err != nil {
		return nil, err
	}
	if err := m.trimJobList(ctx); err != nil {
		log.Printf("trim job list: %v", err)
	}

	snap := job
	m.notify(snap)
	return &snap, nil
}

func (m *Manager) Get(id uuid.UUID) (*Job, bool) {
	if m.rdb == nil {
		return nil, false
	}
	payload, err := m.loadJob(context.Background(), id)
	if err != nil {
		return nil, false
	}
	job := payload.Job
	return &job, true
}

func (m *Manager) List(limit int) []Job {
	if m.rdb == nil {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	ctx := context.Background()
	ids, err := m.rdb.LRange(ctx, redisKeyJobs, 0, int64(limit-1)).Result()
	if err != nil {
		return nil
	}
	out := make([]Job, 0, len(ids))
	for _, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		payload, err := m.loadJob(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, payload.Job)
	}
	return out
}

func (m *Manager) workerLoop(ctx context.Context, runner Runner) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		res, err := m.rdb.BRPop(ctx, 5*time.Second, redisKeyQueue).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			log.Printf("import worker brpop: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(res) < 2 {
			continue
		}
		jobID, err := uuid.Parse(res[1])
		if err != nil {
			continue
		}
		m.processJob(ctx, runner, jobID)
	}
}

func (m *Manager) processJob(ctx context.Context, runner Runner, jobID uuid.UUID) {
	payload, err := m.loadJob(ctx, jobID)
	if err != nil {
		log.Printf("import job %s load: %v", jobID, err)
		return
	}

	payload.Job.Status = JobStatusRunning
	payload.Job.Message = "导入进行中"
	payload.Job.UpdatedAt = time.Now().UTC()
	_ = m.saveJob(ctx, *payload)
	m.notify(payload.Job)

	for i, sf := range payload.Files {
		m.updateItem(ctx, jobID, i, func(it *Item) {
			it.Status = ItemStatusProcessing
		})

		raw, err := m.minio.Get(ctx, sf.ObjectKey)
		if err != nil {
			m.failItem(ctx, jobID, i, err)
			continue
		}

		fileCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		result, err := runner.ImportFile(fileCtx, payload.Job.CollectionID, FileInput{
			Filename:   sf.Filename,
			SourceType: sf.SourceType,
			Raw:        raw,
			StagingKey: sf.ObjectKey,
		})
		cancel()

		_ = m.minio.Delete(ctx, sf.ObjectKey)

		if err != nil {
			m.failItem(ctx, jobID, i, err)
			continue
		}

		docID := result.Document.ID
		status := ItemStatusDone
		switch result.Outcome {
		case ingestion.OutcomeSkipped:
			status = ItemStatusSkipped
		case ingestion.OutcomeUpdated:
			status = ItemStatusUpdated
		}
		m.updateItem(ctx, jobID, i, func(it *Item) {
			it.Status = status
			it.Outcome = result.Outcome
			if result.Message != "" && status == ItemStatusSkipped {
				it.Error = result.Message
			}
			if docID != uuid.Nil {
				it.DocumentID = &docID
				it.Title = result.Document.Title
				it.ChunkCount = result.Document.ChunkCount
			}
			it.VectorsStored = result.VectorsStored
		})
		m.bumpProgress(ctx, jobID, true)
	}

	m.finalizeJob(ctx, jobID)
}

func (m *Manager) failItem(ctx context.Context, jobID uuid.UUID, index int, err error) {
	log.Printf("import job %s item %d: %v", jobID, index, err)
	m.updateItem(ctx, jobID, index, func(it *Item) {
		it.Status = ItemStatusFailed
		it.Error = err.Error()
	})
	m.bumpProgress(ctx, jobID, false)
}

func (m *Manager) finalizeJob(ctx context.Context, jobID uuid.UUID) {
	payload, err := m.loadJob(ctx, jobID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	j := &payload.Job
	j.CompletedAt = &now
	j.Progress = percent(j.Done+j.Failed, j.Total)
	switch {
	case j.Done == j.Total:
		j.Status = JobStatusCompleted
		j.Message = "全部导入完成"
	case j.Done == 0:
		j.Status = JobStatusFailed
		j.Message = "导入失败"
	default:
		j.Status = JobStatusPartial
		j.Message = "部分导入完成"
	}
	j.UpdatedAt = now
	_ = m.saveJob(ctx, *payload)
	m.notify(*j)
}

func (m *Manager) bumpProgress(ctx context.Context, jobID uuid.UUID, success bool) {
	payload, err := m.loadJob(ctx, jobID)
	if err != nil {
		return
	}
	if success {
		payload.Job.Done++
	} else {
		payload.Job.Failed++
	}
	payload.Job.Progress = percent(payload.Job.Done+payload.Job.Failed, payload.Job.Total)
	payload.Job.UpdatedAt = time.Now().UTC()
	_ = m.saveJob(ctx, *payload)
	m.notify(payload.Job)
}

func (m *Manager) updateItem(ctx context.Context, jobID uuid.UUID, index int, fn func(*Item)) {
	payload, err := m.loadJob(ctx, jobID)
	if err != nil || index < 0 || index >= len(payload.Job.Items) {
		return
	}
	fn(&payload.Job.Items[index])
	payload.Job.UpdatedAt = time.Now().UTC()
	_ = m.saveJob(ctx, *payload)
	m.notify(payload.Job)
}

func (m *Manager) notify(job Job) {
	if m.broadcast != nil {
		m.broadcast.BroadcastImport(job)
	}
}

type redisJobPayload struct {
	Job   Job          `json:"job"`
	Files []storedFile `json:"files"`
}

func (m *Manager) saveJob(ctx context.Context, payload redisJobPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	key := redisKeyJobPref + payload.Job.ID.String()
	if err := m.rdb.Set(ctx, key, raw, jobTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (m *Manager) loadJob(ctx context.Context, id uuid.UUID) (*redisJobPayload, error) {
	raw, err := m.rdb.Get(ctx, redisKeyJobPref+id.String()).Bytes()
	if err != nil {
		return nil, err
	}
	var payload redisJobPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (m *Manager) trimJobList(ctx context.Context) error {
	n, err := m.rdb.LLen(ctx, redisKeyJobs).Result()
	if err != nil {
		return err
	}
	if n <= int64(m.maxJobs) {
		return nil
	}
	return m.rdb.LTrim(ctx, redisKeyJobs, 0, int64(m.maxJobs-1)).Err()
}
