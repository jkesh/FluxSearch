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

	if h.stores.Retrieval == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"query":   req.Query,
			"results": []searchResultItem{},
			"message": "retrieval unavailable",
		})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(req.CollectionID, bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	hits, mode, err := h.stores.Retrieval.Search(c.Request.Context(), collectionID, req.Query, topK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]searchResultItem, 0, len(hits))
	for _, hit := range hits {
		results = append(results, searchResultItem{
			ChunkID:       hit.ChunkID,
			DocumentID:    hit.DocumentID,
			DocumentTitle: hit.DocumentTitle,
			Content:       hit.Content,
			Score:         hit.Score,
			Page:          hit.Page,
			Section:       hit.Section,
		})
	}

	collectionName := "fluxsearch_default"
	if h.stores.Postgres != nil {
		if coll, err := h.stores.Postgres.GetCollectionByID(c.Request.Context(), collectionID); err == nil {
			collectionName = coll.MilvusCollection
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"query":      req.Query,
		"top_k":      topK,
		"collection": collectionName,
		"mode":       mode,
		"count":      len(results),
		"results":    results,
	})
}
