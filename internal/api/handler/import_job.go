package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/fluxsearch/fluxsearch/internal/bootstrap"
	"github.com/fluxsearch/fluxsearch/internal/importqueue"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) CreateImportJob(c *gin.Context) {
	if h.stores.Ingestion == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "postgres unavailable"})
		return
	}

	if h.stores.ImportQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "import queue unavailable (redis/minio required)"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form required"})
		return
	}
	defer form.RemoveAll()

	headers := form.File["files"]
	if len(headers) == 0 {
		headers = form.File["file"]
	}
	if len(headers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one file is required"})
		return
	}

	collectionID, err := uuid.Parse(defaultStr(c.PostForm("collection_id"), bootstrap.DefaultCollectionID()))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid collection_id"})
		return
	}

	sourceType := c.PostForm("source_type")
	inputs := make([]importqueue.FileInput, 0, len(headers))
	for _, header := range headers {
		f, err := header.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		raw, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		inputs = append(inputs, importqueue.FileInput{
			Filename:   header.Filename,
			SourceType: sourceType,
			Raw:        raw,
		})
	}

	job, err := h.stores.ImportQueue.Enqueue(c.Request.Context(), collectionID, inputs)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *Handler) GetImportJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	job, ok := h.stores.ImportQueue.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

func (h *Handler) ListImportJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	c.JSON(http.StatusOK, gin.H{"jobs": h.stores.ImportQueue.List(limit)})
}
