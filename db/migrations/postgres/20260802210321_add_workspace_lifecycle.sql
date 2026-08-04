-- +goose Up
ALTER TABLE workspaces
    ADD COLUMN status                text NOT NULL DEFAULT 'active',
    ADD COLUMN timezone              text NOT NULL DEFAULT 'UTC',
    ADD COLUMN deletion_requested_at timestamptz,
    ADD COLUMN purge_after           timestamptz;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_status_check
        CHECK (status IN ('active', 'pending_deletion')),
    ADD CONSTRAINT workspaces_deletion_check
        CHECK (status <> 'pending_deletion' OR (deletion_requested_at IS NOT NULL AND purge_after IS NOT NULL));

CREATE INDEX workspaces_purge_after_idx
    ON workspaces (purge_after) WHERE status = 'pending_deletion';

-- +goose Down
DROP INDEX workspaces_purge_after_idx;

ALTER TABLE workspaces
    DROP CONSTRAINT workspaces_deletion_check,
    DROP CONSTRAINT workspaces_status_check;

ALTER TABLE workspaces
    DROP COLUMN purge_after,
    DROP COLUMN deletion_requested_at,
    DROP COLUMN timezone,
    DROP COLUMN status;
