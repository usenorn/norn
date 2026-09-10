-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN description_doc jsonb;

ALTER TABLE workspace_issue_comments
    ADD COLUMN body_doc jsonb;

CREATE TABLE workspace_issue_description_revisions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL,
    issue_id          uuid NOT NULL,
    issue_version     integer NOT NULL,
    doc               jsonb NOT NULL,
    markdown          text NOT NULL,
    author_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    source            text NOT NULL DEFAULT 'person',
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_description_revisions_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_description_revisions_version_check
        CHECK (issue_version > 0),
    CONSTRAINT workspace_issue_description_revisions_source_check
        CHECK (source IN ('person', 'agent', 'import', 'intake', 'restore'))
);

CREATE INDEX workspace_issue_description_revisions_issue_idx
    ON workspace_issue_description_revisions (issue_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX workspace_issue_description_revisions_version_key
    ON workspace_issue_description_revisions (issue_id, issue_version);

-- +goose Down
DROP TABLE workspace_issue_description_revisions;

ALTER TABLE workspace_issue_comments
    DROP COLUMN body_doc;

ALTER TABLE workspace_issues
    DROP COLUMN description_doc;
