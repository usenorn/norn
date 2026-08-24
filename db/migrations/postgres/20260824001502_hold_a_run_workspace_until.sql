-- +goose Up
ALTER TABLE workspace_executions
    ADD COLUMN keep_until timestamptz;

-- +goose Down
ALTER TABLE workspace_executions
    DROP COLUMN keep_until;
