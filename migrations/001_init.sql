-- FluxSearch V0: collections / documents / chunks
-- PostgreSQL 16+

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ── collections ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS collections (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    embedding_model     VARCHAR(128) NOT NULL DEFAULT 'text-embedding-3-small',
    milvus_collection   VARCHAR(255) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT collections_name_unique UNIQUE (name)
);

-- ── documents ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    title           VARCHAR(512) NOT NULL,
    source_type     VARCHAR(32) NOT NULL,
    source_uri      VARCHAR(1024),
    content_hash    VARCHAR(64),
    version         INT NOT NULL DEFAULT 1,
    status          VARCHAR(32) NOT NULL DEFAULT 'pending',
    error_message   TEXT,
    chunk_count     INT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    indexed_at      TIMESTAMPTZ,
    CONSTRAINT documents_status_check CHECK (
        status IN ('pending', 'processing', 'indexed', 'failed')
    ),
    CONSTRAINT documents_source_type_check CHECK (
        source_type IN ('pdf', 'markdown', 'docx', 'txt', 'html')
    )
);

-- ── chunks ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS chunks (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id             UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_version        INT NOT NULL DEFAULT 1,
    chunk_index             INT NOT NULL,
    chunk_hash              VARCHAR(64) NOT NULL,
    content                 TEXT NOT NULL,
    token_count             INT NOT NULL DEFAULT 0,
    page                    INT,
    section                 VARCHAR(512),
    metadata                JSONB NOT NULL DEFAULT '{}',
    embedding_model_version VARCHAR(128),
    status                  VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chunks_status_check CHECK (
        status IN ('active', 'stale', 'deleted')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_document_chunk_index_active
    ON chunks(document_id, chunk_index)
    WHERE status = 'active';

-- ── indexes ──────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_documents_collection ON documents(collection_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_status ON chunks(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_document_hash_active
    ON chunks(document_id, chunk_hash)
    WHERE status = 'active';

-- ── updated_at trigger ───────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS collections_updated_at ON collections;
CREATE TRIGGER collections_updated_at
    BEFORE UPDATE ON collections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS documents_updated_at ON documents;
CREATE TRIGGER documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ── migration tracking ───────────────────────────────────
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     VARCHAR(64) PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO schema_migrations (version)
VALUES ('001_init')
ON CONFLICT (version) DO NOTHING;

-- ── default collection (V0) ────────────────────────────────
INSERT INTO collections (id, name, description, embedding_model, milvus_collection)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'default',
    'Default knowledge base',
    'text-embedding-3-small',
    'fluxsearch_default'
)
ON CONFLICT (name) DO NOTHING;

COMMIT;
