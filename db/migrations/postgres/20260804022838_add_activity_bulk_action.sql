-- +goose Up
ALTER TABLE workspace_issue_activity
    ADD COLUMN bulk_action_id uuid REFERENCES workspace_bulk_actions (id) ON DELETE SET NULL;

CREATE INDEX workspace_issue_activity_bulk_action_idx
    ON workspace_issue_activity (bulk_action_id) WHERE bulk_action_id IS NOT NULL;

-- +goose Down
DROP INDEX workspace_issue_activity_bulk_action_idx;

ALTER TABLE workspace_issue_activity
    DROP COLUMN bulk_action_id;
