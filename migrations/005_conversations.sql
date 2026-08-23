-- FluxSearch: conversations / messages for chat history

BEGIN;

CREATE TABLE IF NOT EXISTS conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    title           VARCHAR(255) NOT NULL DEFAULT '',
    status          VARCHAR(32) NOT NULL DEFAULT 'active',
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT conversations_status_check CHECK (
        status IN ('active', 'archived')
    )
);

CREATE TABLE IF NOT EXISTS messages (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id  UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role             VARCHAR(32) NOT NULL,
    content          TEXT NOT NULL,
    sources          JSONB NOT NULL DEFAULT '[]',
    metadata         JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_role_check CHECK (
        role IN ('user', 'assistant', 'system')
    )
);

CREATE INDEX IF NOT EXISTS idx_conversations_collection
    ON conversations(collection_id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated_at
    ON conversations(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversations_status_updated
    ON conversations(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_conversation
    ON messages(conversation_id, created_at);

DROP TRIGGER IF EXISTS conversations_updated_at ON conversations;
CREATE TRIGGER conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO schema_migrations (version)
VALUES ('005_conversations')
ON CONFLICT (version) DO NOTHING;

COMMIT;
