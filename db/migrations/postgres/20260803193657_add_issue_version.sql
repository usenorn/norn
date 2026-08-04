-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN version        integer NOT NULL DEFAULT 1,
    ADD COLUMN field_versions jsonb   NOT NULL DEFAULT '{}',
    ADD CONSTRAINT workspace_issues_version_check CHECK (version > 0);

-- +goose Down
ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_version_check,
    DROP COLUMN field_versions,
    DROP COLUMN version;
