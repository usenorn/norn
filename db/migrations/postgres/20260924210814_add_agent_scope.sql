-- +goose Up
ALTER TABLE workspace_agents
    ADD COLUMN scope text NOT NULL DEFAULT 'member'
        CHECK (scope IN ('member', 'project', 'workspace')),
    ADD COLUMN project_id uuid
        CONSTRAINT workspace_agents_project_fkey REFERENCES workspace_projects (id),
    ADD CONSTRAINT workspace_agents_scope_project_check
        CHECK ((scope = 'project') = (project_id IS NOT NULL));

CREATE INDEX workspace_agents_project_idx
    ON workspace_agents (project_id) WHERE project_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS workspace_agents_project_idx;

ALTER TABLE workspace_agents
    DROP CONSTRAINT IF EXISTS workspace_agents_scope_project_check,
    DROP COLUMN IF EXISTS project_id,
    DROP COLUMN IF EXISTS scope;
