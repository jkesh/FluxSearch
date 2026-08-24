package milvus

import (
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func sparseFromVector(vec embedding.SparseVector) (entity.SparseEmbedding, error) {
	if len(vec) == 0 {
		return entity.NewSliceSparseEmbedding(nil, nil)
	}
	positions := make([]uint32, 0, len(vec))
	values := make([]float32, 0, len(vec))
	for id, value := range vec {
		if value == 0 {
			continue
		}
		positions = append(positions, id)
		values = append(values, value)
	}
	return entity.NewSliceSparseEmbedding(positions, values)
}
