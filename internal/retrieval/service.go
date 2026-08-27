package retrieval

import (
	"context"
	"fmt"
	"sort"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/fluxsearch/fluxsearch/internal/rerank"
	"github.com/fluxsearch/fluxsearch/internal/retrieval/bm25"
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
	bm25     *bm25.Index
}

func NewService(
	pg *pgstore.Store,
	milvus *milvusstore.Store,
	embedder embedding.Embedder,
	reranker rerank.Reranker,
	settings *settings.Manager,
	bm25Index *bm25.Index,
) *Service {
	return &Service{
		pg:       pg,
		milvus:   milvus,
		embedder: embedder,
		reranker: reranker,
		settings: settings,
		bm25:     bm25Index,
	}
}

func (s *Service) Configure(embedder embedding.Embedder, reranker rerank.Reranker) {
	s.embedder = embedder
	s.reranker = reranker
}

func (s *Service) BM25() *bm25.Index {
	return s.bm25
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
	if topK <= 0 {
		topK = 5
	}

	cfg := s.settings.EffectiveSearchSettings()
	recallK := cfg.SearchRecallK
	if recallK < topK {
		recallK = topK
	}

	mode := cfg.EffectiveSearchMode()
	var hits []document.SearchHit
	var err error
	resultMode := mode

	switch mode {
	case settings.SearchModeSparseHybrid:
		hits, resultMode, err = s.searchSparseHybrid(ctx, collectionID, query, recallK)
	case settings.SearchModeDenseBM25:
		hits, resultMode, err = s.searchDenseBM25(ctx, collectionID, query, recallK, cfg.SearchDenseWeight, cfg.SearchBM25Weight)
	default:
		hits, resultMode, err = s.searchDense(ctx, collectionID, query, recallK)
	}
	if err != nil {
		return nil, "", err
	}
	if len(hits) == 0 {
		return nil, resultMode, nil
	}

	// Collapse chunk-level hits to one candidate per document before rerank / top_k.
	hits = aggregateHitsByDocument(hits)

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
		resultMode = resultMode + "+rerank"
	} else if len(hits) > topK {
		hits = hits[:topK]
	}

	return s.enrichHits(ctx, hits), resultMode, nil
}

func (s *Service) searchDense(ctx context.Context, collectionID uuid.UUID, query string, recallK int) ([]document.SearchHit, string, error) {
	if s.embedder == nil || s.milvus == nil {
		return nil, "", fmt.Errorf("embedding or milvus unavailable")
	}
	collectionName, err := s.resolveCollectionName(ctx, collectionID)
	if err != nil {
		return nil, "", err
	}
	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, "", fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, settings.SearchModeDense, nil
	}
	hits, err := s.milvus.Search(ctx, collectionName, vectors[0], recallK)
	if err != nil {
		return nil, "", err
	}
	return hits, settings.SearchModeDense, nil
}

func (s *Service) searchSparseHybrid(ctx context.Context, collectionID uuid.UUID, query string, recallK int) ([]document.SearchHit, string, error) {
	if s.embedder == nil || s.milvus == nil {
		return nil, "", fmt.Errorf("embedding or milvus unavailable")
	}
	collectionName, err := s.resolveCollectionName(ctx, collectionID)
	if err != nil {
		return nil, "", err
	}

	if hybrid, ok := embedding.AsHybrid(s.embedder); ok {
		vectors, err := hybrid.EmbedHybrid(ctx, []string{query})
		if err != nil {
			return nil, "", fmt.Errorf("embed query: %w", err)
		}
		if len(vectors) > 0 {
			hits, err := s.milvus.HybridSearch(ctx, collectionName, vectors[0].Dense, vectors[0].Sparse, recallK, recallK)
			if err != nil {
				return nil, "", err
			}
			return hits, settings.SearchModeSparseHybrid, nil
		}
	}
	// Fallback to dense if hybrid embedder unavailable.
	return s.searchDense(ctx, collectionID, query, recallK)
}

func (s *Service) searchDenseBM25(
	ctx context.Context,
	collectionID uuid.UUID,
	query string,
	recallK int,
	denseW, bm25W float64,
) ([]document.SearchHit, string, error) {
	denseW, bm25W = settings.NormalizeFusionWeights(denseW, bm25W)

	var denseHits []document.SearchHit
	if denseW > 0 {
		if s.embedder == nil || s.milvus == nil {
			return nil, "", fmt.Errorf("embedding or milvus unavailable")
		}
		hits, _, err := s.searchDense(ctx, collectionID, query, recallK)
		if err != nil {
			return nil, "", err
		}
		denseHits = hits
	}

	var bm25Hits []document.SearchHit
	if bm25W > 0 && s.bm25 != nil {
		raw := s.bm25.Search(query, collectionID, recallK)
		bm25Hits = make([]document.SearchHit, 0, len(raw))
		for _, h := range raw {
			bm25Hits = append(bm25Hits, document.SearchHit{
				ChunkID:    h.ChunkID,
				DocumentID: h.DocumentID,
				Content:    h.Content,
				Score:      h.Score,
				Page:       h.Page,
				Section:    h.Section,
			})
		}
	}

	switch {
	case denseW > 0 && bm25W > 0:
		fused := fuseWeightedRRF([][]document.SearchHit{denseHits, bm25Hits}, []float64{denseW, bm25W}, recallK)
		return fused, settings.SearchModeDenseBM25, nil
	case bm25W > 0:
		return bm25Hits, "bm25", nil
	default:
		return denseHits, settings.SearchModeDense, nil
	}
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

// aggregateHitsByDocument keeps the highest-scoring chunk per document.
func aggregateHitsByDocument(hits []document.SearchHit) []document.SearchHit {
	if len(hits) == 0 {
		return hits
	}
	best := make(map[uuid.UUID]document.SearchHit, len(hits))
	for _, hit := range hits {
		prev, ok := best[hit.DocumentID]
		if !ok || hit.Score > prev.Score {
			best[hit.DocumentID] = hit
		}
	}
	out := make([]document.SearchHit, 0, len(best))
	for _, hit := range best {
		out = append(out, hit)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}
