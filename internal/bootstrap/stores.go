package bootstrap

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/chunker"
	"github.com/fluxsearch/fluxsearch/internal/chat"
	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/fluxsearch/fluxsearch/internal/importqueue"
	"github.com/fluxsearch/fluxsearch/internal/ingestion"
	"github.com/fluxsearch/fluxsearch/internal/llm"
	"github.com/fluxsearch/fluxsearch/internal/reindex"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	miniostore "github.com/fluxsearch/fluxsearch/internal/storage/minio"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
	redisstore "github.com/fluxsearch/fluxsearch/internal/storage/redis"
	"github.com/google/uuid"
)

const defaultMilvusCollection = "fluxsearch_default"

type Stores struct {
	Postgres    *pgstore.Store
	Milvus      *milvusstore.Store
	Redis       *redisstore.Client
	Minio       *miniostore.Store
	Embedder    embedding.Embedder
	LLM         llm.Client
	Chat        *chat.Service
	Ingestion   *ingestion.Service
	Settings    *settings.Manager
	Reindex     *reindex.Coordinator
	ImportQueue *importqueue.Manager
	Config      config.Config
}

func InitStores(ctx context.Context) Stores {
	settingsMgr := settings.NewManager()
	cfg := settingsMgr.ToConfig()
	stores := Stores{
		Settings: settingsMgr,
		Config:   cfg,
		Reindex:  reindex.NewCoordinator(),
	}

	stores.initEmbedder(cfg)
	stores.initLLM(cfg)
	stores.initPostgres(ctx, cfg)
	stores.initMilvus(ctx, cfg)
	stores.initRedis(ctx, cfg)
	stores.initMinio(ctx, cfg)
	stores.initImportQueue()
	stores.initIngestion(cfg)
	stores.initChat()

	return stores
}

func (s *Stores) initLLM(cfg config.Config) {
	// LLM is loaded from settings manager (app.settings.json), not env-only config.
	client, err := llm.NewFromSettings(s.Settings.Get())
	if err != nil {
		log.Printf("llm init failed: %v", err)
		s.LLM = nil
		return
	}
	s.LLM = client
	if client != nil {
		log.Printf("llm ready: model=%s", client.Model())
	} else {
		log.Printf("llm disabled (configure in settings)")
	}
}

func (s *Stores) initChat() {
	if s.Postgres == nil {
		s.Chat = nil
		return
	}
	if s.Chat == nil {
		s.Chat = chat.NewService(s.Postgres, s.Milvus, s.Embedder, s.LLM, s.Settings)
		return
	}
	s.Chat.Configure(s.Embedder, s.LLM)
}

func (s *Stores) WireImportQueue(broadcaster importqueue.Broadcaster) {
	if s.ImportQueue == nil {
		return
	}
	s.ImportQueue.SetBroadcaster(broadcaster)
	s.ImportQueue.StartWorker(context.Background(), s)
}

func (s *Stores) initEmbedder(cfg config.Config) {
	embedder, err := embedding.NewFromConfig(cfg)
	if err != nil {
		log.Printf("embedding init failed: %v", err)
		s.Embedder = nil
		return
	}
	s.Embedder = embedder
	if embedder != nil {
		log.Printf("embedding ready: provider=%s model=%s dim=%d",
			embedder.Provider(), embedder.Model(), embedder.Dimension())
	} else {
		log.Printf("embedding disabled (set provider in settings)")
	}
}

func (s *Stores) initPostgres(ctx context.Context, cfg config.Config) {
	pgCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	pg, err := pgstore.NewStore(pgCtx, cfg)
	if err != nil {
		log.Printf("postgres unavailable: %v", err)
		return
	}
	s.Postgres = pg
	log.Printf("postgres connected: %s:%d/%s", cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB)
}

func (s *Stores) initMilvus(ctx context.Context, cfg config.Config) {
	if s.Milvus != nil {
		_ = s.Milvus.Close()
	}

	mvCtx, mvCancel := context.WithTimeout(ctx, 15*time.Second)
	defer mvCancel()
	idx := s.Settings.MilvusIndexConfig()
	mv, err := milvusstore.NewStore(mvCtx, cfg, idx)
	if err != nil {
		log.Printf("milvus unavailable: %v", err)
		s.Milvus = nil
		return
	}
	s.Milvus = mv
	log.Printf("milvus connected: %s:%d index=%s metric=%s",
		cfg.MilvusHost, cfg.MilvusPort, idx.IndexType, idx.Metric)

	collCtx, collCancel := context.WithTimeout(ctx, 30*time.Second)
	defer collCancel()
	if err := mv.EnsureCollection(collCtx, defaultMilvusCollection); err != nil {
		log.Printf("milvus ensure collection: %v", err)
	} else {
		log.Printf("milvus collection ready: %s (dim=%d)", defaultMilvusCollection, mv.VectorDim())
	}
}

func (s *Stores) initRedis(ctx context.Context, cfg config.Config) {
	rCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rdb, err := redisstore.NewClient(rCtx, cfg)
	if err != nil {
		log.Printf("redis unavailable: %v", err)
		return
	}
	s.Redis = rdb
	log.Printf("redis connected: %s:%d", cfg.RedisHost, cfg.RedisPort)
}

func (s *Stores) initMinio(ctx context.Context, cfg config.Config) {
	mCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	mv, err := miniostore.NewStore(mCtx, cfg)
	if err != nil {
		log.Printf("minio unavailable: %v", err)
		return
	}
	s.Minio = mv
	log.Printf("minio connected: %s bucket=%s", cfg.MinioEndpoint, mv.Bucket())
}

func (s *Stores) initImportQueue() {
	if s.Redis == nil {
		log.Printf("import queue: redis required")
		return
	}
	s.ImportQueue = importqueue.NewManager(s.Redis.Raw(), s.Minio)
}

func (s *Stores) initIngestion(cfg config.Config) {
	chunkOpts := chunker.Options{MaxChars: cfg.ChunkMaxChars, Overlap: cfg.ChunkOverlap}
	dedup := ingestion.DedupConfigFromSettings(s.Settings.Get())
	var objects ingestion.ObjectStore
	if s.Minio != nil {
		objects = s.Minio
	}
	if s.Postgres == nil {
		s.Ingestion = nil
		return
	}
	if s.Ingestion == nil {
		s.Ingestion = ingestion.NewService(
			s.Postgres,
			s.Milvus,
			s.Embedder,
			chunkOpts,
		)
	}
	s.Ingestion.Configure(s.Embedder, chunkOpts, dedup, objects)
}

func (s *Stores) ReloadRuntime(plan settings.ReindexPlan) error {
	cfg := s.Settings.ToConfig()
	s.Config = cfg
	s.initEmbedder(cfg)
	s.initLLM(cfg)

	if plan.RecreateCollection {
		s.initMilvus(context.Background(), cfg)
	} else if s.Milvus != nil {
		s.Milvus.SetIndexConfig(s.Settings.MilvusIndexConfig())
	}

	s.initIngestion(cfg)
	s.initChat()
	return nil
}

func (s *Stores) RecreateMilvusCollection(ctx context.Context) error {
	if s.Milvus == nil {
		return fmt.Errorf("milvus unavailable")
	}
	idx := s.Settings.MilvusIndexConfig()
	s.Milvus.SetIndexConfig(idx)
	s.Milvus.SetVectorDim(s.Config.EmbeddingDim)
	log.Printf("recreating milvus collection %s (%s)", defaultMilvusCollection, idx.IndexSignature())
	return s.Milvus.RecreateCollection(ctx, defaultMilvusCollection)
}

func (s *Stores) StartReindex(plan settings.ReindexPlan) bool {
	if s.Reindex == nil {
		s.Reindex = reindex.NewCoordinator()
	}
	collectionID, _ := uuid.Parse(DefaultCollectionID())
	runner := reindex.BootstrapRunner{
		RecreateFn: s.RecreateMilvusCollection,
		ListIDsFn: func(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if s.Postgres == nil {
				return nil, fmt.Errorf("postgres unavailable")
			}
			return s.Postgres.ListDocumentIDs(ctx, id)
		},
		RechunkFn: func(ctx context.Context, docID uuid.UUID) error {
			if s.Ingestion == nil {
				return fmt.Errorf("ingestion unavailable")
			}
			return s.Ingestion.RechunkDocument(ctx, docID)
		},
		ReembedFn: func(ctx context.Context, docID uuid.UUID) error {
			if s.Ingestion == nil {
				return fmt.Errorf("ingestion unavailable")
			}
			return s.Ingestion.ReembedDocument(ctx, docID)
		},
		CollectionID: collectionID,
	}
	return s.Reindex.Start(runner, plan)
}

func (s *Stores) ReindexView() settings.ReindexView {
	if s.Reindex == nil {
		return settings.ReindexView{}
	}
	return s.Reindex.View()
}

func (s *Stores) EmbeddingStatus() (ready bool, status string) {
	if s.Embedder == nil {
		cfg := s.Settings.Get()
		if cfg.EmbeddingProvider == "" {
			return false, "disabled"
		}
		return false, "not initialized"
	}
	return true, fmt.Sprintf("%s / %s (dim=%d)",
		s.Embedder.Provider(), s.Embedder.Model(), s.Embedder.Dimension())
}

func (s *Stores) Close() {
	if s.Postgres != nil {
		s.Postgres.Close()
	}
	if s.Milvus != nil {
		_ = s.Milvus.Close()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
}

func DefaultCollectionID() string {
	return document.DefaultCollectionID
}

func (s *Stores) ImportFile(ctx context.Context, collectionID uuid.UUID, in importqueue.FileInput) (ingestion.ImportResult, error) {
	if s.Ingestion == nil {
		return ingestion.ImportResult{}, fmt.Errorf("ingestion unavailable")
	}
	return s.Ingestion.Import(ctx, ingestion.ImportInput{
		CollectionID: collectionID,
		Title:        "",
		Filename:     in.Filename,
		SourceType:   in.SourceType,
		Raw:          in.Raw,
	})
}
