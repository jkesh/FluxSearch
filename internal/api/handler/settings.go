package handler

import (
	"fmt"
	"net/http"

	"github.com/fluxsearch/fluxsearch/internal/settings"
	"github.com/gin-gonic/gin"
)

func (h *Handler) GetSettings(c *gin.Context) {
	ready, status := h.stores.EmbeddingStatus()
	c.JSON(http.StatusOK, h.stores.Settings.PublicView(ready, status, h.stores.ReindexView()))
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var in settings.UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	before := h.stores.Settings.Get()

	if err := h.stores.Settings.Update(in); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	after := h.stores.Settings.Get()
	plan := settings.DetectReindexPlan(before, after)

	if err := h.stores.ReloadRuntime(plan); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":      fmt.Sprintf("settings saved but runtime reload failed: %v", err),
			"settings":     h.stores.Settings.PublicView(false, err.Error(), h.stores.ReindexView()),
			"reload_error": err.Error(),
			"reindex_plan": plan,
		})
		return
	}

	reindexStarted := false
	if plan.Needed {
		reindexStarted = h.stores.StartReindex(plan)
	}

	ready, status := h.stores.EmbeddingStatus()
	msg := "settings saved and applied"
	if plan.Needed {
		if reindexStarted {
			msg = "settings saved; reindex started in background"
		} else {
			msg = "settings saved; reindex already running"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         msg,
		"settings":        h.stores.Settings.PublicView(ready, status, h.stores.ReindexView()),
		"reindex_plan":    plan,
		"reindex_started": reindexStarted,
	})
}
