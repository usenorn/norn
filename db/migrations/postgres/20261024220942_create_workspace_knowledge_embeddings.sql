-- +goose Up
CREATE INDEX workspace_memories_hash_idx ON workspace_memories (workspace_id, content_hash);

CREATE TABLE workspace_knowledge_embeddings (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    model        text NOT NULL,
    content_hash bytea NOT NULL,
    embedding    vector NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, model, content_hash),
    CONSTRAINT workspace_knowledge_embeddings_model_check CHECK (char_length(model) BETWEEN 1 AND 128),
    CONSTRAINT workspace_knowledge_embeddings_hash_check CHECK (octet_length(content_hash) = 16)
);

-- +goose Down
DROP TABLE workspace_knowledge_embeddings;

DROP INDEX workspace_memories_hash_idx;
