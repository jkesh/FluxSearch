-- 存储解析后的原文，支持 rechunk
BEGIN;

ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS content TEXT,
    ADD COLUMN IF NOT EXISTS content_pages JSONB;

INSERT INTO schema_migrations (version)
VALUES ('002_document_content')
ON CONFLICT (version) DO NOTHING;

COMMIT;
