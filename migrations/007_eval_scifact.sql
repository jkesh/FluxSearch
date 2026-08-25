-- Eval collection for BEIR SciFact retrieval benchmark

BEGIN;

INSERT INTO collections (id, name, description, embedding_model, milvus_collection)
VALUES (
    '00000000-0000-0000-0000-0000000000e2',
    'eval-scifact',
    'BEIR SciFact — scientific claim verification retrieval',
    'text-embedding-3-small',
    'fluxsearch_eval_scifact'
)
ON CONFLICT (name) DO NOTHING;

INSERT INTO schema_migrations (version)
VALUES ('007_eval_scifact')
ON CONFLICT (version) DO NOTHING;

COMMIT;
