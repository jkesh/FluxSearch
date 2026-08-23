package milvus

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type VectorRecord struct {
	ChunkID               string
	DocumentID            string
	DocumentVersion       int64
	Content               string
	Vector                []float32
	Page                  int64
	Section               string
	EmbeddingModelVersion string
}

func (s *Store) InsertVectors(ctx context.Context, collection string, records []VectorRecord) error {
	if len(records) == 0 {
		return nil
	}

	chunkIDs := make([]string, len(records))
	documentIDs := make([]string, len(records))
	versions := make([]int64, len(records))
	contents := make([]string, len(records))
	vectors := make([][]float32, len(records))
	pages := make([]int64, len(records))
	sections := make([]string, len(records))
	modelVersions := make([]string, len(records))

	for i, r := range records {
		if len(r.Vector) != s.dim {
			return fmt.Errorf("vector dim mismatch: got %d want %d", len(r.Vector), s.dim)
		}
		chunkIDs[i] = r.ChunkID
		documentIDs[i] = r.DocumentID
		versions[i] = r.DocumentVersion
		contents[i] = truncate(r.Content, 8192)
		vectors[i] = r.Vector
		pages[i] = r.Page
		sections[i] = truncate(r.Section, 512)
		modelVersions[i] = r.EmbeddingModelVersion
	}

	_, err := s.client.Insert(ctx, collection, "",
		entity.NewColumnVarChar(FieldChunkID, chunkIDs),
		entity.NewColumnVarChar(FieldDocumentID, documentIDs),
		entity.NewColumnInt64(FieldDocumentVersion, versions),
		entity.NewColumnVarChar(FieldContent, contents),
		entity.NewColumnFloatVector(FieldDenseVector, s.dim, vectors),
		entity.NewColumnInt64(FieldPage, pages),
		entity.NewColumnVarChar(FieldSection, sections),
		entity.NewColumnVarChar(FieldEmbeddingModelVersion, modelVersions),
	)
	if err != nil {
		return fmt.Errorf("insert vectors: %w", err)
	}
	return nil
}

func (s *Store) Search(ctx context.Context, collection string, vector []float32, topK int) ([]document.SearchHit, error) {
	if len(vector) != s.dim {
		return nil, fmt.Errorf("query vector dim mismatch: got %d want %d", len(vector), s.dim)
	}
	if topK <= 0 {
		topK = 5
	}

	idx := s.idx.Normalized()
	sp, err := s.buildSearchParam(idx)
	if err != nil {
		return nil, fmt.Errorf("search param: %w", err)
	}

	vectors := []entity.Vector{entity.FloatVector(vector)}
	results, err := s.client.Search(
		ctx,
		collection,
		nil,
		"",
		[]string{FieldChunkID, FieldDocumentID, FieldContent, FieldPage, FieldSection},
		vectors,
		FieldDenseVector,
		idx.MetricType(),
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	hits, err := parseSearchResults(&results[0])
	if err != nil {
		return nil, err
	}
	return filterByScoreThreshold(hits, idx.ScoreThreshold), nil
}

func (s *Store) buildSearchParam(idx IndexConfig) (entity.SearchParam, error) {
	switch strings.ToLower(idx.IndexType) {
	case IndexTypeHNSW:
		return entity.NewIndexHNSWSearchParam(idx.HNSWEf)
	default:
		return entity.NewIndexIvfFlatSearchParam(idx.NProbe)
	}
}

func filterByScoreThreshold(hits []document.SearchHit, threshold float32) []document.SearchHit {
	if threshold <= 0 || len(hits) == 0 {
		return hits
	}
	out := make([]document.SearchHit, 0, len(hits))
	for _, h := range hits {
		if h.Score >= threshold {
			out = append(out, h)
		}
	}
	return out
}

func (s *Store) DeleteByDocument(ctx context.Context, collection, documentID string) error {
	expr := fmt.Sprintf(`document_id == "%s"`, documentID)
	return s.client.Delete(ctx, collection, "", expr)
}

func parseSearchResults(result *client.SearchResult) ([]document.SearchHit, error) {
	var hits []document.SearchHit

	chunkCol := result.Fields.GetColumn(FieldChunkID)
	docCol := result.Fields.GetColumn(FieldDocumentID)
	contentCol := result.Fields.GetColumn(FieldContent)
	pageCol := result.Fields.GetColumn(FieldPage)
	sectionCol := result.Fields.GetColumn(FieldSection)

	if chunkCol == nil || docCol == nil {
		return nil, nil
	}

	chunkStrs := chunkCol.(*entity.ColumnVarChar).Data()
	docStrs := docCol.(*entity.ColumnVarChar).Data()

	var contents []string
	if contentCol != nil {
		contents = contentCol.(*entity.ColumnVarChar).Data()
	}
	var pages []int64
	if pageCol != nil {
		pages = pageCol.(*entity.ColumnInt64).Data()
	}
	var sections []string
	if sectionCol != nil {
		sections = sectionCol.(*entity.ColumnVarChar).Data()
	}

	for i, idStr := range chunkStrs {
		chunkID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		docID, err := uuid.Parse(docStrs[i])
		if err != nil {
			continue
		}

		hit := document.SearchHit{
			ChunkID:    chunkID,
			DocumentID: docID,
			Score:      result.Scores[i],
		}
		if i < len(contents) {
			hit.Content = contents[i]
		}
		if i < len(pages) && pages[i] > 0 {
			p := int(pages[i])
			hit.Page = &p
		}
		if i < len(sections) {
			hit.Section = sections[i]
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
