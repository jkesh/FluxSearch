package handler

import (
	"net/http"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListCollections(c *gin.Context) {
	if h.stores.Postgres == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	items, err := h.stores.Postgres.ListCollections(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []document.Collection{}
	}

	c.JSON(http.StatusOK, gin.H{"collections": items})
}
