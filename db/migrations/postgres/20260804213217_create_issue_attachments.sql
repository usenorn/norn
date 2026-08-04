-- +goose Up
CREATE UNIQUE INDEX workspace_issue_comments_id_workspace_key
    ON workspace_issue_comments (id, workspace_id);

CREATE TABLE workspace_issue_attachments (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id           uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id               uuid,
    comment_id             uuid,
    uploaded_by_account_id uuid,
    object_key             text NOT NULL,
    file_name              text NOT NULL,
    content_type           text NOT NULL,
    size_bytes             bigint NOT NULL,
    status                 text NOT NULL DEFAULT 'pending',
    reclaim_after          timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_attachments_status_check
        CHECK (status IN ('pending', 'stored', 'discarded')),
    CONSTRAINT workspace_issue_attachments_size_check
        CHECK (size_bytes >= 0),
    CONSTRAINT workspace_issue_attachments_key_check
        CHECK (object_key <> ''),
    CONSTRAINT workspace_issue_attachments_name_check
        CHECK (file_name <> ''),
    CONSTRAINT workspace_issue_attachments_type_check
        CHECK (content_type <> ''),
    CONSTRAINT workspace_issue_attachments_reclaim_check
        CHECK (status <> 'discarded' OR reclaim_after IS NOT NULL),
    CONSTRAINT workspace_issue_attachments_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id)
        ON DELETE SET NULL (issue_id),
    CONSTRAINT workspace_issue_attachments_comment_fkey
        FOREIGN KEY (comment_id, workspace_id)
        REFERENCES workspace_issue_comments (id, workspace_id)
        ON DELETE SET NULL (comment_id),
    CONSTRAINT workspace_issue_attachments_uploader_fkey
        FOREIGN KEY (workspace_id, uploaded_by_account_id)
        REFERENCES workspace_memberships (workspace_id, account_id)
        ON DELETE SET NULL (uploaded_by_account_id)
);

CREATE UNIQUE INDEX workspace_issue_attachments_object_key
    ON workspace_issue_attachments (object_key);

CREATE INDEX workspace_issue_attachments_issue_idx
    ON workspace_issue_attachments (issue_id, created_at, id)
    WHERE issue_id IS NOT NULL AND status = 'stored';

CREATE INDEX workspace_issue_attachments_comment_idx
    ON workspace_issue_attachments (comment_id, created_at, id)
    WHERE comment_id IS NOT NULL AND status = 'stored';

CREATE INDEX workspace_issue_attachments_reclaim_idx
    ON workspace_issue_attachments (reclaim_after, id)
    WHERE reclaim_after IS NOT NULL;

CREATE INDEX workspace_issue_attachments_orphan_idx
    ON workspace_issue_attachments (id)
    WHERE issue_id IS NULL;

CREATE TABLE workspace_storage_ledger (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    stored_bytes bigint NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_storage_ledger_bytes_check CHECK (stored_bytes >= 0)
);

-- +goose Down
DROP TABLE workspace_storage_ledger;

DROP TABLE workspace_issue_attachments;

DROP INDEX workspace_issue_comments_id_workspace_key;
