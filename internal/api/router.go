package api

import (
	"context"

	"github.com/fluxsearch/fluxsearch/internal/api/handler"
	"github.com/fluxsearch/fluxsearch/internal/api/ws"
	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/fluxsearch/fluxsearch/internal/importqueue"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	hub := ws.NewHub()
	go hub.Run()

	stores := bootstrap.InitStores(context.Background())
	hub.SetChatService(stores.Chat)

	stores.WireImportQueue(importqueue.NotifyFunc(func(job importqueue.Job) {
		hub.BroadcastEvents(map[string]any{
			"type": "import_progress",
			"job":  job,
		})
	}))

	h := handler.New(hub, stores)

	r.GET("/healthz", h.Healthz)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/conversations", h.ListConversations)
		v1.POST("/conversations", h.CreateConversation)
		v1.GET("/conversations/:id", h.GetConversation)
		v1.PATCH("/conversations/:id", h.UpdateConversation)
		v1.DELETE("/conversations/:id", h.DeleteConversation)
		v1.GET("/conversations/:id/messages", h.ListConversationMessages)
		v1.GET("/ws/chat", h.ChatWS)
		v1.GET("/ws/events", h.EventsWS)
		v1.POST("/search", h.Search)
		v1.GET("/documents", h.ListDocuments)
		v1.GET("/documents/:id", h.GetDocument)
		v1.DELETE("/documents/:id", h.DeleteDocument)
		v1.POST("/documents", h.UploadDocument)
		v1.POST("/documents/batch", h.UploadDocumentsBatch)
		v1.POST("/import/jobs", h.CreateImportJob)
		v1.GET("/import/jobs", h.ListImportJobs)
		v1.GET("/import/jobs/:id", h.GetImportJob)
		v1.POST("/documents/:id/rechunk", h.RechunkDocument)
		v1.GET("/system/status", h.SystemStatus)
		v1.GET("/settings", h.GetSettings)
		v1.PUT("/settings", h.UpdateSettings)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "not found"})
	})

	return r
}
