package retrieval

import (
	"context"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/fluxsearch/fluxsearch/internal/rerank"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
	"github.com/google/uuid"
)

type Service struct {
	pg       *pgstore.Store
	milvus   *milvusstore.Store
	embedder embedding.Embedder
	reranker rerank.Reranker
	settings *settings.Manager
}

func NewService(
	pg *pgstore.Store,
	milvus *milvusstore.Store,
	embedder embedding.Embedder,
	reranker rerank.Reranker,
	settings *settings.Manager,
) *Service {
	return &Service{
		pg:       pg,
		milvus:   milvus,
		embedder: embedder,
		reranker: reranker,
		settings: settings,
	}
}

func (s *Service) Configure(embedder embedding.Embedder, reranker rerank.Reranker) {
	s.embedder = embedder
	s.reranker = reranker
}

type Hit struct {
	ChunkID       uuid.UUID
	DocumentID    uuid.UUID
	DocumentTitle string
	Content       string
	Score         float32
	Page          *int
	Section       string
}

func (s *Service) Search(ctx context.Context, collectionID uuid.UUID, query string, topK int) ([]Hit, string, error) {
	if s.embedder == nil || s.milvus == nil {
		return nil, "", fmt.Errorf("embedding or milvus unavailable")
	}
	if topK <= 0 {
		topK = 5
	}

	cfg := s.settings.EffectiveSearchSettings()
	collectionName, err := s.resolveCollectionName(ctx, collectionID)
	if err != nil {
		return nil, "", err
	}

	recallK := cfg.SearchRecallK
	if recallK < topK {
		recallK = topK
	}

	var hits []document.SearchHit
	mode := "dense"

	if cfg.SearchHybridEnabled {
		if hybrid, ok := embedding.AsHybrid(s.embedder); ok {
			vectors, err := hybrid.EmbedHybrid(ctx, []string{query})
			if err != nil {
				return nil, "", fmt.Errorf("embed query: %w", err)
			}
			if len(vectors) > 0 {
				hits, err = s.milvus.HybridSearch(ctx, collectionName, vectors[0].Dense, vectors[0].Sparse, recallK, topK)
				if err != nil {
					return nil, "", err
				}
				mode = "hybrid"
			}
		}
	}

	if mode == "dense" {
		vectors, err := s.embedder.Embed(ctx, []string{query})
		if err != nil {
			return nil, "", fmt.Errorf("embed query: %w", err)
		}
		if len(vectors) == 0 {
			return nil, mode, nil
		}
		hits, err = s.milvus.Search(ctx, collectionName, vectors[0], recallK)
		if err != nil {
			return nil, "", err
		}
	}

	if len(hits) == 0 {
		return nil, mode, nil
	}

	if cfg.SearchRerankEnabled && s.reranker != nil && len(hits) > 1 {
		candidates := make([]rerank.Candidate, len(hits))
		for i, hit := range hits {
			candidates[i] = rerank.Candidate{Index: i, Content: hit.Content}
		}
		ranked, err := s.reranker.Rerank(ctx, query, candidates, topK)
		if err != nil {
			return nil, "", fmt.Errorf("rerank: %w", err)
		}
		reordered := make([]document.SearchHit, 0, len(ranked))
		for _, item := range ranked {
			if item.Index < 0 || item.Index >= len(hits) {
				continue
			}
			hit := hits[item.Index]
			hit.Score = item.Score
			reordered = append(reordered, hit)
		}
		hits = reordered
		mode = mode + "+rerank"
	} else if len(hits) > topK {
		hits = hits[:topK]
	}

	return s.enrichHits(ctx, hits), mode, nil
}

func (s *Service) resolveCollectionName(ctx context.Context, collectionID uuid.UUID) (string, error) {
	collectionName := "fluxsearch_default"
	if s.pg != nil {
		coll, err := s.pg.GetCollectionByID(ctx, collectionID)
		if err != nil {
			return "", fmt.Errorf("get collection: %w", err)
		}
		collectionName = coll.MilvusCollection
	}
	return collectionName, nil
}

func (s *Service) enrichHits(ctx context.Context, hits []document.SearchHit) []Hit {
	docIDs := make([]uuid.UUID, 0, len(hits))
	seen := make(map[uuid.UUID]struct{}, len(hits))
	for _, hit := range hits {
		if _, ok := seen[hit.DocumentID]; ok {
			continue
		}
		seen[hit.DocumentID] = struct{}{}
		docIDs = append(docIDs, hit.DocumentID)
	}

	titles := map[uuid.UUID]string{}
	if s.pg != nil && len(docIDs) > 0 {
		docs, err := s.pg.GetDocumentsByIDs(ctx, docIDs)
		if err == nil {
			for id, doc := range docs {
				titles[id] = doc.Title
			}
		}
	}

	out := make([]Hit, 0, len(hits))
	for _, hit := range hits {
		out = append(out, Hit{
			ChunkID:       hit.ChunkID,
			DocumentID:    hit.DocumentID,
			DocumentTitle: titles[hit.DocumentID],
			Content:       hit.Content,
			Score:         hit.Score,
			Page:          hit.Page,
			Section:       hit.Section,
		})
	}
	return out
}
