-- +goose Up
ALTER TABLE workspace_issue_comments
    ADD COLUMN version integer NOT NULL DEFAULT 1,
    ADD CONSTRAINT workspace_issue_comments_version_check CHECK (version > 0);

-- +goose Down
ALTER TABLE workspace_issue_comments DROP COLUMN version;
