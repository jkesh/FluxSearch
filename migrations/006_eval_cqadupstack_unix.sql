-- Eval collection for BEIR CQADupStack / Unix retrieval benchmark

BEGIN;

INSERT INTO collections (id, name, description, embedding_model, milvus_collection)
VALUES (
    '00000000-0000-0000-0000-0000000000e1',
    'eval-cqadupstack-unix',
    'BEIR CQADupStack Unix — retrieval evaluation corpus',
    'text-embedding-3-small',
    'fluxsearch_eval_cqadupstack_unix'
)
ON CONFLICT (name) DO NOTHING;

INSERT INTO schema_migrations (version)
VALUES ('006_eval_cqadupstack_unix')
ON CONFLICT (version) DO NOTHING;

COMMIT;
