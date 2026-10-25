-- +goose Up
ALTER TABLE workspace_memories
    ADD COLUMN token_id uuid REFERENCES api_tokens (id) ON DELETE SET NULL;

ALTER TABLE workspace_memory_revisions
    ADD COLUMN token_id uuid REFERENCES api_tokens (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE workspace_memory_revisions DROP COLUMN token_id;

ALTER TABLE workspace_memories DROP COLUMN token_id;
