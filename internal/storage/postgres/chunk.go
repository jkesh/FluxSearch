package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
)

func (s *Store) ExistingChunkHashesInCollection(ctx context.Context, collectionID uuid.UUID, hashes []string) (map[string]struct{}, error) {
	if len(hashes) == 0 {
		return map[string]struct{}{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT c.chunk_hash
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE d.collection_id = $1
		  AND c.status = 'active'
		  AND c.chunk_hash = ANY($2)`, collectionID, hashes)
	if err != nil {
		return nil, fmt.Errorf("existing chunk hashes in collection: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("scan chunk hash: %w", err)
		}
		out[hash] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) ExistingChunkHashesForDocument(ctx context.Context, documentID uuid.UUID, hashes []string) (map[string]struct{}, error) {
	if len(hashes) == 0 {
		return map[string]struct{}{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT chunk_hash
		FROM chunks
		WHERE document_id = $1
		  AND status = 'active'
		  AND chunk_hash = ANY($2)`, documentID, hashes)
	if err != nil {
		return nil, fmt.Errorf("existing chunk hashes for document: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("scan chunk hash: %w", err)
		}
		out[hash] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) CreateChunks(ctx context.Context, chunks []document.CreateChunkInput) ([]document.Chunk, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := `
		INSERT INTO chunks (
			document_id, document_version, chunk_index, chunk_hash, content,
			token_count, page, section, metadata, embedding_model_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9::jsonb, NULLIF($10, ''))
		RETURNING ` + chunkSelectCols

	var created []document.Chunk
	for _, in := range chunks {
		meta, err := json.Marshal(in.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal chunk metadata: %w", err)
		}
		if len(meta) == 0 || string(meta) == "null" {
			meta = []byte("{}")
		}

		c, err := scanChunk(tx.QueryRow(ctx, q,
			in.DocumentID, in.DocumentVersion, in.ChunkIndex, in.ChunkHash, in.Content,
			in.TokenCount, in.Page, in.Section, meta, in.EmbeddingModelVersion,
		))
		if err != nil {
			return nil, fmt.Errorf("insert chunk: %w", err)
		}
		created = append(created, c)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit chunks: %w", err)
	}
	return created, nil
}

func (s *Store) UpdateChunksEmbeddingVersion(ctx context.Context, chunkIDs []uuid.UUID, version string) error {
	if len(chunkIDs) == 0 || version == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE chunks SET embedding_model_version = $2
		WHERE id = ANY($1) AND status = 'active'`, chunkIDs, version)
	if err != nil {
		return fmt.Errorf("update chunk embedding version: %w", err)
	}
	return nil
}

func (s *Store) GetChunksByIDs(ctx context.Context, ids []uuid.UUID) ([]document.Chunk, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT `+chunkSelectCols+`
		FROM chunks
		WHERE id = ANY($1) AND status = 'active'`, ids)
	if err != nil {
		return nil, fmt.Errorf("get chunks by ids: %w", err)
	}
	defer rows.Close()

	var chunks []document.Chunk
	for rows.Next() {
		c, err := scanChunkRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) ListChunksByDocument(ctx context.Context, documentID uuid.UUID) ([]document.Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+chunkSelectCols+`
		FROM chunks
		WHERE document_id = $1 AND status = 'active'
		ORDER BY chunk_index`, documentID)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []document.Chunk
	for rows.Next() {
		c, err := scanChunkRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) CountChunks(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM chunks WHERE status = 'active'`).Scan(&n)
	return n, err
}

// BM25Chunk is a minimal active chunk row for BM25 index rebuild.
type BM25Chunk struct {
	ID           uuid.UUID
	DocumentID   uuid.UUID
	CollectionID uuid.UUID
	Content      string
	Page         *int
	Section      string
}

func (s *Store) ListActiveChunksForBM25(ctx context.Context) ([]BM25Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.document_id, d.collection_id, c.content, c.page, COALESCE(c.section, '')
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.status = 'active'
		ORDER BY c.created_at`)
	if err != nil {
		return nil, fmt.Errorf("list active chunks for bm25: %w", err)
	}
	defer rows.Close()

	var out []BM25Chunk
	for rows.Next() {
		var ch BM25Chunk
		if err := rows.Scan(&ch.ID, &ch.DocumentID, &ch.CollectionID, &ch.Content, &ch.Page, &ch.Section); err != nil {
			return nil, fmt.Errorf("scan bm25 chunk: %w", err)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (s *Store) MarkChunksStaleByDocument(ctx context.Context, documentID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE chunks SET status = 'stale' WHERE document_id = $1 AND status = 'active'`, documentID)
	if err != nil {
		return fmt.Errorf("mark chunks stale: %w", err)
	}
	return nil
}
