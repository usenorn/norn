-- +goose Up
ALTER TABLE workspaces
    ADD COLUMN agent_instructions text NOT NULL DEFAULT '';

ALTER TABLE workspace_projects
    ADD COLUMN agent_instructions text NOT NULL DEFAULT '';

ALTER TABLE workspace_agents
    ADD COLUMN agent_instructions text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE workspace_agents
    DROP COLUMN agent_instructions;

ALTER TABLE workspace_projects
    DROP COLUMN agent_instructions;

ALTER TABLE workspaces
    DROP COLUMN agent_instructions;
