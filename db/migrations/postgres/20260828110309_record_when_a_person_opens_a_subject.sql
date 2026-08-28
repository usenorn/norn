-- +goose Up
CREATE TABLE workspace_subject_views (
    workspace_id    uuid NOT NULL,
    account_id      uuid NOT NULL,
    subject_kind    text NOT NULL,
    subject_id      uuid NOT NULL,
    first_viewed_at timestamptz NOT NULL DEFAULT now(),
    last_viewed_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id, subject_kind, subject_id),
    CONSTRAINT workspace_subject_views_subject_kind_check
        CHECK (subject_kind IN ('issue', 'project', 'team')),
    CONSTRAINT workspace_subject_views_order_check
        CHECK (last_viewed_at >= first_viewed_at),
    CONSTRAINT workspace_subject_views_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_subject_views_subject_idx
    ON workspace_subject_views (workspace_id, subject_kind, subject_id);

-- +goose Down
DROP TABLE workspace_subject_views;
