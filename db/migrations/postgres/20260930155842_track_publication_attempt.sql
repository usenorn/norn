-- +goose Up
ALTER TABLE workspace_execution_changes
    ADD COLUMN publication_attempt integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT workspace_execution_changes_publication_attempt_check
        CHECK (publication_attempt >= 0);

-- +goose Down
ALTER TABLE workspace_execution_changes
    DROP CONSTRAINT workspace_execution_changes_publication_attempt_check,
    DROP COLUMN publication_attempt;
