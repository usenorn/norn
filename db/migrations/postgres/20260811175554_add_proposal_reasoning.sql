-- +goose Up
ALTER TABLE workspace_agent_proposals
    ADD COLUMN reasoning jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE workspace_agent_proposals DROP COLUMN reasoning;
