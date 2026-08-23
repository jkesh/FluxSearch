package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) DeleteDocument(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (s *Store) UpdateDocumentMetadata(ctx context.Context, id uuid.UUID, metadata map[string]any) error {
	meta, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE documents SET metadata = $2::jsonb, updated_at = NOW() WHERE id = $1`, id, meta)
	if err != nil {
		return fmt.Errorf("update document metadata: %w", err)
	}
	return nil
}

func (s *Store) FindDocumentByContentHash(ctx context.Context, collectionID uuid.UUID, hash string) (document.Document, error) {
	q := `SELECT ` + documentSelectCols + `
		FROM documents
		WHERE collection_id = $1 AND content_hash = $2
		ORDER BY updated_at DESC
		LIMIT 1`
	row := s.pool.QueryRow(ctx, q, collectionID, hash)
	d, err := scanDocument(row)
	if err != nil {
		return document.Document{}, fmt.Errorf("find document by content hash: %w", err)
	}
	return d, nil
}

func (s *Store) FindDocumentBySourceURI(ctx context.Context, collectionID uuid.UUID, sourceURI string) (document.Document, error) {
	q := `SELECT ` + documentSelectCols + `
		FROM documents
		WHERE collection_id = $1 AND source_uri = $2
		ORDER BY updated_at DESC
		LIMIT 1`
	row := s.pool.QueryRow(ctx, q, collectionID, sourceURI)
	d, err := scanDocument(row)
	if err != nil {
		return document.Document{}, fmt.Errorf("find document by source uri: %w", err)
	}
	return d, nil
}

func (s *Store) UpdateDocumentContent(ctx context.Context, id uuid.UUID, in document.UpdateDocumentContentInput) (document.Document, error) {
	pages, err := json.Marshal(in.ContentPages)
	if err != nil {
		return document.Document{}, fmt.Errorf("marshal content_pages: %w", err)
	}
	if len(pages) == 0 || string(pages) == "null" {
		pages = []byte("[]")
	}

	q := `
		UPDATE documents
		SET title = $2,
		    source_type = $3,
		    content_hash = NULLIF($4, ''),
		    content = NULLIF($5, ''),
		    content_pages = $6::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING ` + documentSelectCols

	row := s.pool.QueryRow(ctx, q, id, in.Title, in.SourceType, in.ContentHash, in.Content, pages)
	d, err := scanDocument(row)
	if err != nil {
		return document.Document{}, fmt.Errorf("update document content: %w", err)
	}
	return d, nil
}

func (s *Store) CreateDocument(ctx context.Context, in document.CreateDocumentInput) (document.Document, error) {
	metadata, err := json.Marshal(in.Metadata)
	if err != nil {
		return document.Document{}, fmt.Errorf("marshal metadata: %w", err)
	}
	if len(metadata) == 0 || string(metadata) == "null" {
		metadata = []byte("{}")
	}

	pages, err := json.Marshal(in.ContentPages)
	if err != nil {
		return document.Document{}, fmt.Errorf("marshal content_pages: %w", err)
	}
	if len(pages) == 0 || string(pages) == "null" {
		pages = []byte("[]")
	}

	q := `
		INSERT INTO documents (
			collection_id, title, source_type, source_uri, content_hash,
			content, content_pages, metadata
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7::jsonb, $8::jsonb)
		RETURNING ` + documentSelectCols

	row := s.pool.QueryRow(ctx, q,
		in.CollectionID, in.Title, in.SourceType, in.SourceURI, in.ContentHash,
		in.Content, pages, metadata,
	)
	d, err := scanDocument(row)
	if err != nil {
		return document.Document{}, fmt.Errorf("create document: %w", err)
	}
	return d, nil
}

func (s *Store) GetDocument(ctx context.Context, id uuid.UUID) (document.Document, error) {
	q := `SELECT ` + documentSelectCols + ` FROM documents WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	d, err := scanDocument(row)
	if err != nil {
		return document.Document{}, fmt.Errorf("get document: %w", err)
	}
	return d, nil
}

func (s *Store) ListDocuments(ctx context.Context, collectionID uuid.UUID, limit, offset int) ([]document.DocumentListItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	q := `SELECT ` + documentListCols + `
		FROM documents
		WHERE collection_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, q, collectionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []document.DocumentListItem
	for rows.Next() {
		item, err := scanDocumentListItemRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document list item: %w", err)
		}
		docs = append(docs, item)
	}
	return docs, rows.Err()
}

func (s *Store) UpdateDocumentStatus(ctx context.Context, id uuid.UUID, status, errMsg string) error {
	const q = `
		UPDATE documents
		SET status = $2, error_message = NULLIF($3, ''), updated_at = NOW()
		WHERE id = $1`

	_, err := s.pool.Exec(ctx, q, id, status, errMsg)
	if err != nil {
		return fmt.Errorf("update document status: %w", err)
	}
	return nil
}

func (s *Store) MarkDocumentIndexed(ctx context.Context, id uuid.UUID, chunkCount int) error {
	const q = `
		UPDATE documents
		SET status = $2, chunk_count = $3, indexed_at = NOW(), updated_at = NOW(), error_message = NULL
		WHERE id = $1`

	_, err := s.pool.Exec(ctx, q, id, document.StatusIndexed, chunkCount)
	if err != nil {
		return fmt.Errorf("mark document indexed: %w", err)
	}
	return nil
}

func (s *Store) BumpDocumentVersion(ctx context.Context, id uuid.UUID) (int, error) {
	var version int
	err := s.pool.QueryRow(ctx, `
		UPDATE documents
		SET version = version + 1, updated_at = NOW()
		WHERE id = $1
		RETURNING version`, id).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("bump document version: %w", err)
	}
	return version, nil
}

func (s *Store) CountDocuments(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents`).Scan(&n)
	return n, err
}

func (s *Store) ListDocumentIDs(ctx context.Context, collectionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id FROM documents
		WHERE collection_id = $1
		ORDER BY created_at ASC`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("list document ids: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan document id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) GetDocumentsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]document.Document, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]document.Document{}, nil
	}

	q := `SELECT ` + documentSelectCols + ` FROM documents WHERE id = ANY($1)`
	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("get documents by ids: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]document.Document, len(ids))
	for rows.Next() {
		d, err := scanDocumentRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		out[d.ID] = d
	}
	return out, rows.Err()
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
