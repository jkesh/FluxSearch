package reindex

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	"github.com/google/uuid"
)

type Status struct {
	Running   bool     `json:"running"`
	Total     int      `json:"total"`
	Done      int      `json:"done"`
	Failed    int      `json:"failed"`
	LastError string   `json:"last_error,omitempty"`
	Message   string   `json:"message,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
}

type Runner interface {
	RecreateMilvusCollection(ctx context.Context) error
	ListDocumentIDs(ctx context.Context, collectionID uuid.UUID) ([]uuid.UUID, error)
	RechunkDocument(ctx context.Context, id uuid.UUID) error
	ReembedDocument(ctx context.Context, id uuid.UUID) error
	DefaultCollectionID() uuid.UUID
}

type Coordinator struct {
	mu     sync.RWMutex
	status Status
}

func NewCoordinator() *Coordinator {
	return &Coordinator{}
}

func (c *Coordinator) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Coordinator) View() settings.ReindexView {
	s := c.Status()
	return settings.ReindexView{
		Running:   s.Running,
		Total:     s.Total,
		Done:      s.Done,
		Failed:    s.Failed,
		LastError: s.LastError,
		Message:   s.Message,
	}
}

func (c *Coordinator) Start(runner Runner, plan settings.ReindexPlan) bool {
	if !plan.Needed {
		return false
	}

	c.mu.Lock()
	if c.status.Running {
		c.mu.Unlock()
		return false
	}
	c.status = Status{
		Running: true,
		Message: "重处理已开始",
		Reasons: plan.Reasons,
	}
	c.mu.Unlock()

	go c.run(context.Background(), runner, plan)
	return true
}

func (c *Coordinator) run(ctx context.Context, runner Runner, plan settings.ReindexPlan) {
	defer func() {
		c.mu.Lock()
		c.status.Running = false
		c.mu.Unlock()
	}()

	if plan.RecreateCollection {
		recreateCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err := runner.RecreateMilvusCollection(recreateCtx)
		cancel()
		if err != nil {
			c.setError(err)
			return
		}
	}

	collectionID := runner.DefaultCollectionID()
	listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	ids, err := runner.ListDocumentIDs(listCtx, collectionID)
	cancel()
	if err != nil {
		c.setError(err)
		return
	}

	c.mu.Lock()
	c.status.Total = len(ids)
	c.status.Done = 0
	c.status.Failed = 0
	c.mu.Unlock()

	if len(ids) == 0 {
		c.mu.Lock()
		c.status.Message = "无文档需要重处理"
		c.mu.Unlock()
		return
	}

	for _, id := range ids {
		docCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		var err error
		if plan.RechunkAll {
			err = runner.RechunkDocument(docCtx, id)
		} else if plan.ReembedAll {
			err = runner.ReembedDocument(docCtx, id)
		}
		cancel()

		c.mu.Lock()
		if err != nil {
			c.status.Failed++
			c.status.LastError = err.Error()
			log.Printf("reindex document %s: %v", id, err)
		} else {
			c.status.Done++
		}
		c.mu.Unlock()
	}

	c.mu.Lock()
	c.status.Message = "重处理完成"
	if c.status.Failed > 0 {
		c.status.Message = "重处理完成（部分失败）"
	}
	c.mu.Unlock()
}

func (c *Coordinator) setError(err error) {
	c.mu.Lock()
	c.status.LastError = err.Error()
	c.status.Message = "重处理失败"
	c.mu.Unlock()
	log.Printf("reindex failed: %v", err)
}

// BootstrapRunner adapts bootstrap.Stores for reindex.Coordinator
type BootstrapRunner struct {
	RecreateFn   func(context.Context) error
	ListIDsFn    func(context.Context, uuid.UUID) ([]uuid.UUID, error)
	RechunkFn    func(context.Context, uuid.UUID) error
	ReembedFn    func(context.Context, uuid.UUID) error
	CollectionID uuid.UUID
}

func (r BootstrapRunner) RecreateMilvusCollection(ctx context.Context) error {
	return r.RecreateFn(ctx)
}

func (r BootstrapRunner) ListDocumentIDs(ctx context.Context, collectionID uuid.UUID) ([]uuid.UUID, error) {
	return r.ListIDsFn(ctx, collectionID)
}

func (r BootstrapRunner) RechunkDocument(ctx context.Context, id uuid.UUID) error {
	return r.RechunkFn(ctx, id)
}

func (r BootstrapRunner) ReembedDocument(ctx context.Context, id uuid.UUID) error {
	return r.ReembedFn(ctx, id)
}

func (r BootstrapRunner) DefaultCollectionID() uuid.UUID {
	if r.CollectionID != uuid.Nil {
		return r.CollectionID
	}
	id, _ := uuid.Parse(document.DefaultCollectionID)
	return id
}
