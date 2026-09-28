-- +goose Up
ALTER TABLE workspace_agents
    ADD COLUMN execution text NOT NULL DEFAULT 'runner'
        CHECK (execution IN ('runner', 'hosted'));

-- +goose Down
ALTER TABLE workspace_agents
    DROP COLUMN IF EXISTS execution;
