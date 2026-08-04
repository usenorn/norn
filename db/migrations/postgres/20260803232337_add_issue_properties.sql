-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN description         text NOT NULL DEFAULT '',
    ADD COLUMN priority            text NOT NULL DEFAULT 'none',
    ADD COLUMN assignee_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    ADD COLUMN estimate            integer,
    ADD COLUMN due_on              date,
    ADD COLUMN state_entered_at    timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN completed_at        timestamptz;

ALTER TABLE workspace_issues
    ADD CONSTRAINT workspace_issues_priority_check
        CHECK (priority IN ('urgent', 'high', 'medium', 'low', 'none')),
    ADD CONSTRAINT workspace_issues_estimate_check
        CHECK (estimate IS NULL OR estimate > 0),
    ADD CONSTRAINT workspace_issues_description_check
        CHECK (length(description) <= 20000);

UPDATE workspace_issues i
SET completed_at = i.updated_at
FROM workspace_workflow_states s
WHERE s.id = i.state_id AND s.category = 'complete';

CREATE INDEX workspace_issues_assignee_idx ON workspace_issues (assignee_account_id);
CREATE INDEX workspace_issues_due_idx ON workspace_issues (due_on) WHERE due_on IS NOT NULL;

-- +goose Down
DROP INDEX workspace_issues_due_idx;
DROP INDEX workspace_issues_assignee_idx;

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_description_check,
    DROP CONSTRAINT workspace_issues_estimate_check,
    DROP CONSTRAINT workspace_issues_priority_check;

ALTER TABLE workspace_issues
    DROP COLUMN completed_at,
    DROP COLUMN state_entered_at,
    DROP COLUMN due_on,
    DROP COLUMN estimate,
    DROP COLUMN assignee_account_id,
    DROP COLUMN priority,
    DROP COLUMN description;
