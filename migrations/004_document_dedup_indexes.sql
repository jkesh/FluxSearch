-- Indexes for document dedup lookups (collection-scoped)

BEGIN;

CREATE INDEX IF NOT EXISTS idx_documents_collection_content_hash
    ON documents (collection_id, content_hash)
    WHERE content_hash IS NOT NULL AND content_hash <> '';

CREATE INDEX IF NOT EXISTS idx_documents_collection_source_uri
    ON documents (collection_id, source_uri)
    WHERE source_uri IS NOT NULL AND source_uri <> '';

INSERT INTO schema_migrations (version)
VALUES ('004_document_dedup_indexes')
ON CONFLICT (version) DO NOTHING;

COMMIT;
