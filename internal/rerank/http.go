package rerank

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

type HTTPConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	BatchSize int
}

type httpReranker struct {
	baseURL   string
	apiKey    string
	model     string
	batchSize int
	client    *http.Client
}

func NewHTTP(cfg HTTPConfig) (Reranker, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("rerank base url is required")
	}
	model := cfg.Model
	if model == "" {
		model = "bge-reranker-v2-m3"
	}
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 32
	}
	return &httpReranker{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:    cfg.APIKey,
		model:     model,
		batchSize: batch,
		client:    &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (r *httpReranker) Model() string { return r.model }

func (r *httpReranker) Rerank(ctx context.Context, query string, candidates []Candidate, topK int) ([]Result, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	if topK <= 0 || topK > len(candidates) {
		topK = len(candidates)
	}

	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = c.Content
	}

	body := map[string]any{
		"model":     r.model,
		"query":     query,
		"documents": docs,
		"top_k":     topK,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/rerank", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rerank api %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Data []struct {
			Index int     `json:"index"`
			Score float32 `json:"score"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse rerank response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("rerank api error: %s", parsed.Error.Message)
	}

	out := make([]Result, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(candidates) {
			continue
		}
		out = append(out, Result{
			Index: candidates[item.Index].Index,
			Score: item.Score,
		})
	}
	return out, nil
}
