package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatibleConfig struct {
	Provider       string
	BaseURL        string
	APIKey         string
	Model          string
	Dimension      int
	BatchSize       int
	MaxLength       int
	HybridSupported bool
}

type openAICompatible struct {
	provider        string
	baseURL         string
	apiKey          string
	model           string
	dimension       int
	batchSize       int
	maxLength       int
	hybridSupported bool
	client          *http.Client
}

func NewOpenAICompatible(cfg OpenAICompatibleConfig) (Embedder, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("embedding base url is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 16
	}
	return &openAICompatible{
		provider:        cfg.Provider,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		dimension:       cfg.Dimension,
		batchSize:       batch,
		maxLength:       cfg.MaxLength,
		hybridSupported: cfg.HybridSupported,
		client:          &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (e *openAICompatible) SupportsHybrid() bool { return e.hybridSupported }

func (e *openAICompatible) Provider() string { return e.provider }
func (e *openAICompatible) Model() string    { return e.model }
func (e *openAICompatible) Dimension() int   { return e.dimension }

func (e *openAICompatible) Embed(ctx context.Context, texts []string) ([][]float32, error) {
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

func (e *openAICompatible) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body := map[string]any{
		"model": e.model,
		"input": texts,
	}
	if e.dimension > 0 {
		body["dimensions"] = e.dimension
	}
	if e.maxLength > 0 {
		body["max_length"] = e.maxLength
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	log.Printf("embedding request: provider=%s model=%s texts=%d max_length=%d url=%s/embeddings",
		e.provider, e.model, len(texts), e.maxLength, e.baseURL)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embedding api %d: %s", resp.StatusCode, string(raw))
	}

	log.Printf("embedding response: provider=%s model=%s vectors=%d status=%d",
		e.provider, e.model, len(texts), resp.StatusCode)

	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("embedding api error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: got %d want %d", len(parsed.Data), len(texts))
	}

	vectors := make([][]float32, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			return nil, fmt.Errorf("invalid embedding index: %d", item.Index)
		}
		if e.dimension > 0 && len(item.Embedding) != e.dimension {
			return nil, fmt.Errorf("vector dim mismatch: got %d want %d", len(item.Embedding), e.dimension)
		}
		vectors[item.Index] = item.Embedding
	}
	return vectors, nil
}

func (e *openAICompatible) EmbedHybrid(ctx context.Context, texts []string) ([]HybridVector, error) {
	if !e.hybridSupported {
		return nil, fmt.Errorf("hybrid embedding not supported by provider %s", e.provider)
	}
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([]HybridVector, 0, len(texts))
	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := e.embedHybridBatch(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

func (e *openAICompatible) embedHybridBatch(ctx context.Context, texts []string) ([]HybridVector, error) {
	body := map[string]any{
		"model":          e.model,
		"input":          texts,
		"return_sparse":  true,
	}
	if e.dimension > 0 {
		body["dimensions"] = e.dimension
	}
	if e.maxLength > 0 {
		body["max_length"] = e.maxLength
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hybrid embedding request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hybrid embedding api %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Data []struct {
			Index     int                `json:"index"`
			Embedding []float32          `json:"embedding"`
			Sparse    map[string]float32 `json:"sparse"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse hybrid embedding response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("hybrid embedding api error: %s", parsed.Error.Message)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("hybrid embedding count mismatch: got %d want %d", len(parsed.Data), len(texts))
	}

	vectors := make([]HybridVector, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(vectors) {
			return nil, fmt.Errorf("invalid hybrid embedding index: %d", item.Index)
		}
		if e.dimension > 0 && len(item.Embedding) != e.dimension {
			return nil, fmt.Errorf("vector dim mismatch: got %d want %d", len(item.Embedding), e.dimension)
		}
		vectors[item.Index] = HybridVector{
			Dense:  item.Embedding,
			Sparse: parseSparseWeights(item.Sparse),
		}
	}
	return vectors, nil
}
