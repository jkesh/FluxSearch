package handler

import (
	"net/http"
	"strconv"

	"github.com/fluxsearch/fluxsearch/internal/api/ws"
	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/events"
	"github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	hub    *ws.Hub
	stores bootstrap.Stores
}

func New(hub *ws.Hub, stores bootstrap.Stores) *Handler {
	return &Handler{hub: hub, stores: stores}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "fluxsearch-api",
	})
}

func (h *Handler) ChatWS(c *gin.Context) {
	h.hub.ServeChat(c.Writer, c.Request)
}

func (h *Handler) EventsWS(c *gin.Context) {
	h.hub.ServeEvents(c.Writer, c.Request)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	collectionID, err := uuid.Parse(c.DefaultQuery("collection_id", bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	docs, err := h.stores.Postgres.ListDocuments(c.Request.Context(), collectionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if docs == nil {
		docs = []document.DocumentListItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"collection_id": collectionID,
		"documents":     docs,
		"limit":         limit,
		"offset":        offset,
	})
}

func (h *Handler) GetDocument(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}

	doc, err := h.stores.Postgres.GetDocument(c.Request.Context(), id)
	if err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	chunks, err := h.stores.Postgres.ListChunksByDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if chunks == nil {
		chunks = []document.Chunk{}
	}

	c.JSON(http.StatusOK, gin.H{
		"document": doc,
		"chunks":   chunks,
	})
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	if h.stores.Ingestion == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}

	if err := h.stores.Ingestion.DeleteDocument(c.Request.Context(), id); err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.stores.Events != nil {
		_ = h.stores.Events.Publish(c.Request.Context(), events.DocumentDeleted(id.String()))
	}

	c.JSON(http.StatusOK, gin.H{"message": "document deleted", "document_id": id})
}

func defaultStr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
