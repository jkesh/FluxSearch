package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/fluxsearch/fluxsearch/internal/importqueue"
	"github.com/fluxsearch/fluxsearch/internal/ingestion"
	"github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) UploadDocument(c *gin.Context) {
	if h.stores.Ingestion == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.uploadMultipart(c)
		return
	}
	h.uploadJSON(c)
}

func (h *Handler) uploadJSON(c *gin.Context) {
	var req struct {
		Title        string         `json:"title"`
		Content      string         `json:"content" binding:"required"`
		SourceType   string         `json:"source_type"`
		CollectionID string         `json:"collection_id"`
		Metadata     map[string]any `json:"metadata"`
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

	result, err := h.stores.Ingestion.Import(c.Request.Context(), ingestion.ImportInput{
		CollectionID: collectionID,
		Title:        req.Title,
		Filename:     req.Title,
		SourceType:   req.SourceType,
		Raw:          []byte(req.Content),
		Metadata:     req.Metadata,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, importResponse(result))
}

func (h *Handler) uploadMultipart(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(c.PostForm("collection_id"), bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	result, err := h.stores.Ingestion.Import(c.Request.Context(), ingestion.ImportInput{
		CollectionID: collectionID,
		Title:        c.PostForm("title"),
		Filename:     file.Filename,
		SourceType:   c.PostForm("source_type"),
		Raw:          raw,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, importResponse(result))
}

func (h *Handler) UploadDocumentsBatch(c *gin.Context) {
	if h.stores.Ingestion == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form required"})
		return
	}
	defer form.RemoveAll()

	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one file is required (field: files)"})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(c.PostForm("collection_id"), bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	ctx := c.Request.Context()
	items := make([]gin.H, 0, len(files))
	succeeded := 0

	for _, file := range files {
		item := gin.H{"filename": file.Filename}
		f, err := file.Open()
		if err != nil {
			item["ok"] = false
			item["error"] = err.Error()
			items = append(items, item)
			continue
		}
		raw, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			item["ok"] = false
			item["error"] = err.Error()
			items = append(items, item)
			continue
		}

		result, err := h.stores.Ingestion.Import(ctx, ingestion.ImportInput{
			CollectionID: collectionID,
			Filename:     file.Filename,
			SourceType:   c.PostForm("source_type"),
			Raw:          raw,
		})
		if err != nil {
			item["ok"] = false
			item["error"] = err.Error()
			items = append(items, item)
			continue
		}

		succeeded++
		item["ok"] = true
		item["document_id"] = result.Document.ID
		item["title"] = result.Document.Title
		item["status"] = result.Document.Status
		item["chunk_count"] = result.Document.ChunkCount
		item["vectors_stored"] = result.VectorsStored
		item["outcome"] = defaultOutcome(result.Outcome)
		item["message"] = resultMessage(result)
		items = append(items, item)
	}

	status := http.StatusCreated
	if succeeded == 0 {
		status = http.StatusBadRequest
	} else if succeeded < len(files) {
		status = http.StatusMultiStatus
	}

	c.JSON(status, gin.H{
		"total":     len(files),
		"succeeded": succeeded,
		"failed":    len(files) - succeeded,
		"items":     items,
		"message":   batchImportMessage(succeeded, len(files)),
	})
}

func batchImportMessage(succeeded, total int) string {
	if succeeded == total {
		return "all documents imported"
	}
	if succeeded == 0 {
		return "batch import failed"
	}
	return "batch import completed with errors"
}

func (h *Handler) RechunkDocument(c *gin.Context) {
	if h.stores.Ingestion == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}

	if c.Query("async") == "true" {
		if h.stores.ImportQueue == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "import queue unavailable"})
			return
		}
		if err := h.stores.ImportQueue.EnqueueRechunk(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"message":     "rechunk queued",
			"document_id": id,
			"async":       true,
		})
		return
	}

	result, err := h.stores.Ingestion.Rechunk(c.Request.Context(), id)
	if err != nil {
		if postgres.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"document_id":     result.Document.ID,
		"title":           result.Document.Title,
		"status":          result.Document.Status,
		"version":         result.Document.Version,
		"chunk_count":     result.Document.ChunkCount,
		"vectors_stored":  result.VectorsStored,
		"message":         rechunkMessage(result.VectorsStored),
		"chunks":          result.Chunks,
	})
}

func (h *Handler) ReimportDocument(c *gin.Context) {
	if h.stores.ImportQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "import queue unavailable"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.stores.ImportQueue.EnqueueReimport(c.Request.Context(), id, importqueue.FileInput{
		Filename:   file.Filename,
		SourceType: c.PostForm("source_type"),
		Raw:        raw,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":     "reimport queued",
		"document_id": id,
		"async":       true,
	})
}

func rechunkMessage(vectorsStored bool) string {
	if vectorsStored {
		return "document rechunked and re-embedded"
	}
	return "document rechunked"
}

func importResponse(result ingestion.ImportResult) gin.H {
	resp := gin.H{
		"document_id": result.Document.ID,
		"title":       result.Document.Title,
		"status":      result.Document.Status,
		"version":     result.Document.Version,
		"chunk_count": result.Document.ChunkCount,
		"outcome":     defaultOutcome(result.Outcome),
		"message":     resultMessage(result),
	}
	if result.VectorsStored {
		resp["vectors_stored"] = true
	}
	return resp
}

func defaultOutcome(outcome string) string {
	if outcome == "" {
		return ingestion.OutcomeCreated
	}
	return outcome
}

func resultMessage(result ingestion.ImportResult) string {
	if result.Message != "" {
		return result.Message
	}
	switch result.Outcome {
	case ingestion.OutcomeSkipped:
		return "duplicate document skipped"
	case ingestion.OutcomeUpdated:
		return "document updated and re-indexed"
	default:
		if result.VectorsStored {
			return "document imported, chunked and embedded"
		}
		return "document imported and chunked"
	}
}
