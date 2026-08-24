package embedding

import "context"

// SparseVector maps token id to weight (BGE-M3 lexical weights).
type SparseVector map[uint32]float32

// HybridVector holds dense and sparse embeddings from BGE-M3.
type HybridVector struct {
	Dense  []float32
	Sparse SparseVector
}

// HybridEmbedder extends Embedder with BGE-M3 hybrid output.
type HybridEmbedder interface {
	Embedder
	SupportsHybrid() bool
	EmbedHybrid(ctx context.Context, texts []string) ([]HybridVector, error)
}

func AsHybrid(e Embedder) (HybridEmbedder, bool) {
	h, ok := e.(HybridEmbedder)
	return h, ok && h.SupportsHybrid()
}
