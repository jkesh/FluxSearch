package handler

import (
	"net/http"

	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type searchResultItem struct {
	ChunkID       uuid.UUID `json:"chunk_id"`
	DocumentID    uuid.UUID `json:"document_id"`
	DocumentTitle string    `json:"document_title"`
	Content       string    `json:"content"`
	Score         float32   `json:"score"`
	Page          *int      `json:"page,omitempty"`
	Section       string    `json:"section,omitempty"`
}

func (h *Handler) Search(c *gin.Context) {
	var req struct {
		Query        string `json:"query" binding:"required"`
		TopK         int    `json:"top_k"`
		CollectionID string `json:"collection_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settings := h.stores.Settings.Get()
	topK := req.TopK
	if topK <= 0 {
		topK = settings.SearchTopK
	}
	if topK <= 0 {
		topK = 5
	}

	if h.stores.Embedder == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"query":   req.Query,
			"results": []searchResultItem{},
			"message": "embedding not configured",
		})
		return
	}
	if h.stores.Milvus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"query":   req.Query,
			"results": []searchResultItem{},
			"message": "milvus unavailable",
		})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(req.CollectionID, bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	collectionName := "fluxsearch_default"
	if h.stores.Postgres != nil {
		if coll, err := h.stores.Postgres.GetCollectionByID(c.Request.Context(), collectionID); err == nil {
			collectionName = coll.MilvusCollection
		}
	}

	vectors, err := h.stores.Embedder.Embed(c.Request.Context(), []string{req.Query})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "embed query: " + err.Error()})
		return
	}
	if len(vectors) == 0 {
		c.JSON(http.StatusOK, gin.H{"query": req.Query, "results": []searchResultItem{}, "top_k": topK})
		return
	}

	hits, err := h.stores.Milvus.Search(c.Request.Context(), collectionName, vectors[0], topK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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
	if h.stores.Postgres != nil && len(docIDs) > 0 {
		docs, err := h.stores.Postgres.GetDocumentsByIDs(c.Request.Context(), docIDs)
		if err == nil {
			for id, doc := range docs {
				titles[id] = doc.Title
			}
		}
	}

	results := make([]searchResultItem, 0, len(hits))
	for _, hit := range hits {
		results = append(results, searchResultItem{
			ChunkID:       hit.ChunkID,
			DocumentID:    hit.DocumentID,
			DocumentTitle: titles[hit.DocumentID],
			Content:       hit.Content,
			Score:         hit.Score,
			Page:          hit.Page,
			Section:       hit.Section,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"query":      req.Query,
		"top_k":      topK,
		"collection": collectionName,
		"count":      len(results),
		"results":    results,
	})
}
