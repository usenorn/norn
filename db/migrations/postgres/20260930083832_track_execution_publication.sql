-- +goose Up
ALTER TABLE workspace_execution_changes
    ADD COLUMN publication_state    text NOT NULL DEFAULT '',
    ADD COLUMN publication_step     text NOT NULL DEFAULT '',
    ADD COLUMN publication_error    text NOT NULL DEFAULT '',
    ADD COLUMN published_sha        text NOT NULL DEFAULT '',
    ADD COLUMN publication_revision integer NOT NULL DEFAULT 0,
    ADD COLUMN published_at         timestamptz,
    ADD CONSTRAINT workspace_execution_changes_publication_state_check
        CHECK (publication_state IN ('', 'pending', 'pushed', 'published', 'failed')),
    ADD CONSTRAINT workspace_execution_changes_publication_step_check
        CHECK (publication_step IN ('', 'push', 'pull_request')),
    ADD CONSTRAINT workspace_execution_changes_publication_revision_check
        CHECK (publication_revision >= 0);

-- +goose Down
ALTER TABLE workspace_execution_changes
    DROP CONSTRAINT workspace_execution_changes_publication_revision_check,
    DROP CONSTRAINT workspace_execution_changes_publication_step_check,
    DROP CONSTRAINT workspace_execution_changes_publication_state_check,
    DROP COLUMN published_at,
    DROP COLUMN publication_revision,
    DROP COLUMN published_sha,
    DROP COLUMN publication_error,
    DROP COLUMN publication_step,
    DROP COLUMN publication_state;
