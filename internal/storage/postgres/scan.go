package postgres

import (
	"encoding/json"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/jackc/pgx/v5"
)

const documentSelectCols = `
	id, collection_id, title, source_type,
	COALESCE(source_uri, '') AS source_uri,
	COALESCE(content_hash, '') AS content_hash,
	COALESCE(content, '') AS content,
	COALESCE(content_pages, '[]'::jsonb) AS content_pages,
	version, status,
	COALESCE(error_message, '') AS error_message,
	chunk_count, metadata, created_at, updated_at, indexed_at`

const documentListCols = `
	id, title, source_type,
	COALESCE(source_uri, '') AS source_uri,
	status, chunk_count, version,
	LEFT(COALESCE(content, ''), 200) AS content_preview,
	created_at, updated_at`

func scanDocument(row pgx.Row) (document.Document, error) {
	var d document.Document
	var metaBytes, pagesBytes []byte
	err := row.Scan(
		&d.ID, &d.CollectionID, &d.Title, &d.SourceType, &d.SourceURI, &d.ContentHash,
		&d.Content, &pagesBytes,
		&d.Version, &d.Status, &d.ErrorMessage, &d.ChunkCount, &metaBytes,
		&d.CreatedAt, &d.UpdatedAt, &d.IndexedAt,
	)
	if err != nil {
		return document.Document{}, err
	}
	if err := json.Unmarshal(metaBytes, &d.Metadata); err != nil {
		d.Metadata = map[string]any{}
	}
	if len(pagesBytes) > 0 && string(pagesBytes) != "[]" {
		_ = json.Unmarshal(pagesBytes, &d.ContentPages)
	}
	return d, nil
}

func scanDocumentRows(rows pgx.Rows) (document.Document, error) {
	var d document.Document
	var metaBytes, pagesBytes []byte
	err := rows.Scan(
		&d.ID, &d.CollectionID, &d.Title, &d.SourceType, &d.SourceURI, &d.ContentHash,
		&d.Content, &pagesBytes,
		&d.Version, &d.Status, &d.ErrorMessage, &d.ChunkCount, &metaBytes,
		&d.CreatedAt, &d.UpdatedAt, &d.IndexedAt,
	)
	if err != nil {
		return document.Document{}, err
	}
	if err := json.Unmarshal(metaBytes, &d.Metadata); err != nil {
		d.Metadata = map[string]any{}
	}
	if len(pagesBytes) > 0 && string(pagesBytes) != "[]" {
		_ = json.Unmarshal(pagesBytes, &d.ContentPages)
	}
	return d, nil
}

func scanDocumentListItem(row pgx.Row) (document.DocumentListItem, error) {
	var item document.DocumentListItem
	err := row.Scan(
		&item.ID, &item.Title, &item.SourceType, &item.SourceURI,
		&item.Status, &item.ChunkCount, &item.Version, &item.ContentPreview,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func scanDocumentListItemRows(rows pgx.Rows) (document.DocumentListItem, error) {
	var item document.DocumentListItem
	err := rows.Scan(
		&item.ID, &item.Title, &item.SourceType, &item.SourceURI,
		&item.Status, &item.ChunkCount, &item.Version, &item.ContentPreview,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

const chunkSelectCols = `
	id, document_id, document_version, chunk_index, chunk_hash, content,
	token_count, page,
	COALESCE(section, '') AS section,
	metadata,
	COALESCE(embedding_model_version, '') AS embedding_model_version,
	status, created_at`

func scanChunk(row pgx.Row) (document.Chunk, error) {
	var c document.Chunk
	var metaBytes []byte
	err := row.Scan(
		&c.ID, &c.DocumentID, &c.DocumentVersion, &c.ChunkIndex, &c.ChunkHash, &c.Content,
		&c.TokenCount, &c.Page, &c.Section, &metaBytes, &c.EmbeddingModelVersion, &c.Status, &c.CreatedAt,
	)
	if err != nil {
		return document.Chunk{}, err
	}
	if err := json.Unmarshal(metaBytes, &c.Metadata); err != nil {
		c.Metadata = map[string]any{}
	}
	return c, nil
}

func scanChunkRows(rows pgx.Rows) (document.Chunk, error) {
	var c document.Chunk
	var metaBytes []byte
	err := rows.Scan(
		&c.ID, &c.DocumentID, &c.DocumentVersion, &c.ChunkIndex, &c.ChunkHash, &c.Content,
		&c.TokenCount, &c.Page, &c.Section, &metaBytes, &c.EmbeddingModelVersion, &c.Status, &c.CreatedAt,
	)
	if err != nil {
		return document.Chunk{}, err
	}
	if err := json.Unmarshal(metaBytes, &c.Metadata); err != nil {
		c.Metadata = map[string]any{}
	}
	return c, nil
}
