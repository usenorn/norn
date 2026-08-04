-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN project_id uuid,
    ADD CONSTRAINT workspace_issues_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE SET NULL (project_id);

CREATE INDEX workspace_issues_project_idx
    ON workspace_issues (project_id) WHERE project_id IS NOT NULL;

-- +goose Down
ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_project_fkey,
    DROP COLUMN project_id;
