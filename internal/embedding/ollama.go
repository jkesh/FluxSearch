package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaConfig struct {
	BaseURL   string
	Model     string
	Dimension int
	BatchSize int
}

type ollamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	batchSize int
	client    *http.Client
}

func NewOllama(cfg OllamaConfig) (Embedder, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("ollama base url is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("ollama model is required")
	}
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 16
	}
	return &ollamaEmbedder{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		model:     cfg.Model,
		dimension: cfg.Dimension,
		batchSize: batch,
		client:    &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (e *ollamaEmbedder) Provider() string { return ProviderOllama }
func (e *ollamaEmbedder) Model() string    { return e.model }
func (e *ollamaEmbedder) Dimension() int   { return e.dimension }

func (e *ollamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := e.embedBatch(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

func (e *ollamaEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload, err := json.Marshal(map[string]any{
		"model": e.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama embed %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse ollama response: %w", err)
	}
	if len(parsed.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama embedding count mismatch: got %d want %d", len(parsed.Embeddings), len(texts))
	}
	for i, vec := range parsed.Embeddings {
		if e.dimension > 0 && len(vec) != e.dimension {
			return nil, fmt.Errorf("ollama vector dim mismatch at %d: got %d want %d", i, len(vec), e.dimension)
		}
	}
	return parsed.Embeddings, nil
}
