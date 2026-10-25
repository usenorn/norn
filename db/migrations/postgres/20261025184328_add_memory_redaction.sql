-- +goose Up
ALTER TABLE workspace_memories
    ADD COLUMN redacted boolean NOT NULL DEFAULT false;

ALTER TABLE workspace_memory_revisions
    ADD COLUMN redacted boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_memory_revisions DROP COLUMN redacted;

ALTER TABLE workspace_memories DROP COLUMN redacted;
