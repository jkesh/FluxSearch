package handler

import (
	"net/http"
	"strconv"

	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ListConversations(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	collectionID, err := uuid.Parse(c.DefaultQuery("collection_id", bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, err := h.stores.Postgres.ListConversations(c.Request.Context(), collectionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []conversation.ListItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"collection_id":  collectionID,
		"conversations": items,
		"limit":          limit,
		"offset":         offset,
	})
}

func (h *Handler) CreateConversation(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	var req struct {
		CollectionID string `json:"collection_id"`
		Title        string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(req.CollectionID, bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	conv, err := h.stores.Postgres.CreateConversation(c.Request.Context(), collectionID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"conversation": conv})
}

func (h *Handler) GetConversation(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	conv, err := h.stores.Postgres.GetConversation(c.Request.Context(), id)
	if err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	messages, err := h.stores.Postgres.ListMessages(c.Request.Context(), id, 200, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if messages == nil {
		messages = []conversation.Message{}
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation": conv,
		"messages":     messages,
	})
}

func (h *Handler) UpdateConversation(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	var req struct {
		Title  *string `json:"title"`
		Status *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := h.stores.Postgres.UpdateConversation(c.Request.Context(), id, req.Title, req.Status)
	if err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conversation": conv})
}

func (h *Handler) DeleteConversation(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	if err := h.stores.Postgres.DeleteConversation(c.Request.Context(), id); err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted", "conversation_id": id})
}

func (h *Handler) ListConversationMessages(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := h.stores.Postgres.ListMessages(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if messages == nil {
		messages = []conversation.Message{}
	}

	c.JSON(http.StatusOK, gin.H{
		"conversation_id": id,
		"messages":      messages,
		"limit":           limit,
		"offset":          offset,
	})
}
