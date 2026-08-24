package rerank

import "context"

// Candidate is a passage to score against the query.
type Candidate struct {
	Index   int
	Content string
}

// Result is a reranked candidate with score.
type Result struct {
	Index int
	Score float32
}

// Reranker scores query-passage pairs (cross-encoder).
type Reranker interface {
	Model() string
	Rerank(ctx context.Context, query string, candidates []Candidate, topK int) ([]Result, error)
}
