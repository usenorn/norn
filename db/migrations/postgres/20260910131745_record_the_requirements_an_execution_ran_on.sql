-- +goose Up
ALTER TABLE workspace_executions
    ADD COLUMN description_revision_id uuid
        REFERENCES workspace_issue_description_revisions (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE workspace_executions
    DROP COLUMN description_revision_id;
