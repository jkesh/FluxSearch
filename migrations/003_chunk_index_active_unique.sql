-- Fix rechunk: allow same chunk_index after old rows are marked stale
BEGIN;

ALTER TABLE chunks DROP CONSTRAINT IF EXISTS chunks_document_chunk_index_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_document_chunk_index_active
    ON chunks(document_id, chunk_index)
    WHERE status = 'active';

INSERT INTO schema_migrations (version)
VALUES ('003_chunk_index_active_unique')
ON CONFLICT (version) DO NOTHING;

COMMIT;
