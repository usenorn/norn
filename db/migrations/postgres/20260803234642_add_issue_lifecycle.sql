-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN status                text NOT NULL DEFAULT 'active',
    ADD COLUMN archived_at           timestamptz,
    ADD COLUMN deletion_requested_at timestamptz,
    ADD COLUMN purge_after           timestamptz;

ALTER TABLE workspace_issues
    ADD CONSTRAINT workspace_issues_status_check
        CHECK (status IN ('active', 'archived', 'pending_deletion')),
    ADD CONSTRAINT workspace_issues_archived_check
        CHECK (status <> 'archived' OR archived_at IS NOT NULL),
    ADD CONSTRAINT workspace_issues_deletion_check
        CHECK (status <> 'pending_deletion' OR (deletion_requested_at IS NOT NULL AND purge_after IS NOT NULL));

CREATE INDEX workspace_issues_purge_after_idx
    ON workspace_issues (purge_after) WHERE status = 'pending_deletion';

-- +goose Down
DROP INDEX workspace_issues_purge_after_idx;

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_deletion_check,
    DROP CONSTRAINT workspace_issues_archived_check,
    DROP CONSTRAINT workspace_issues_status_check;

ALTER TABLE workspace_issues
    DROP COLUMN purge_after,
    DROP COLUMN deletion_requested_at,
    DROP COLUMN archived_at,
    DROP COLUMN status;
