-- +goose Up
CREATE TABLE workspace_issue_comments (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL,
    issue_id          uuid NOT NULL,
    parent_comment_id uuid,
    root_marker       uuid GENERATED ALWAYS AS
                          (CASE WHEN parent_comment_id IS NULL THEN id END) STORED,
    author_account_id uuid,
    author_kind       text NOT NULL DEFAULT 'person',
    body              text NOT NULL,
    edited_at         timestamptz,
    deleted_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_comments_kind_check
        CHECK (author_kind IN ('person', 'agent')),
    CONSTRAINT workspace_issue_comments_self_check
        CHECK (parent_comment_id IS NULL OR parent_comment_id <> id),
    CONSTRAINT workspace_issue_comments_body_check
        CHECK (deleted_at IS NOT NULL OR body <> ''),
    CONSTRAINT workspace_issue_comments_tombstone_check
        CHECK (deleted_at IS NULL OR (body = '' AND edited_at IS NULL)),
    CONSTRAINT workspace_issue_comments_root_key UNIQUE (root_marker, issue_id),
    CONSTRAINT workspace_issue_comments_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_comments_author_fkey
        FOREIGN KEY (workspace_id, author_account_id)
        REFERENCES workspace_memberships (workspace_id, account_id)
        ON DELETE SET NULL (author_account_id),
    CONSTRAINT workspace_issue_comments_parent_fkey
        FOREIGN KEY (parent_comment_id, issue_id)
        REFERENCES workspace_issue_comments (root_marker, issue_id) ON DELETE CASCADE
);

CREATE INDEX workspace_issue_comments_thread_idx
    ON workspace_issue_comments (issue_id, created_at, id)
    WHERE parent_comment_id IS NULL;

CREATE INDEX workspace_issue_comments_replies_idx
    ON workspace_issue_comments (parent_comment_id, created_at, id)
    WHERE parent_comment_id IS NOT NULL;

CREATE INDEX workspace_issue_comments_author_idx
    ON workspace_issue_comments (workspace_id, author_account_id);

CREATE TABLE workspace_issue_comment_mentions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id     uuid NOT NULL,
    workspace_id   uuid NOT NULL,
    kind           text NOT NULL,
    account_id     uuid REFERENCES accounts (id) ON DELETE SET NULL,
    team_id        uuid REFERENCES workspace_teams (id) ON DELETE SET NULL,
    mentioned_name text NOT NULL,
    visible        boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_comment_mentions_kind_check
        CHECK (kind IN ('account', 'team')),
    CONSTRAINT workspace_issue_comment_mentions_account_check
        CHECK (account_id IS NULL OR kind = 'account'),
    CONSTRAINT workspace_issue_comment_mentions_team_check
        CHECK (team_id IS NULL OR kind = 'team'),
    CONSTRAINT workspace_issue_comment_mentions_name_check
        CHECK (mentioned_name <> ''),
    CONSTRAINT workspace_issue_comment_mentions_comment_fkey
        FOREIGN KEY (comment_id) REFERENCES workspace_issue_comments (id) ON DELETE CASCADE
);

CREATE INDEX workspace_issue_comment_mentions_comment_idx
    ON workspace_issue_comment_mentions (comment_id, created_at);

CREATE UNIQUE INDEX workspace_issue_comment_mentions_account_key
    ON workspace_issue_comment_mentions (comment_id, account_id) WHERE account_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_issue_comment_mentions_team_key
    ON workspace_issue_comment_mentions (comment_id, team_id) WHERE team_id IS NOT NULL;

CREATE INDEX workspace_issue_comment_mentions_account_idx
    ON workspace_issue_comment_mentions (account_id) WHERE account_id IS NOT NULL;

CREATE TABLE workspace_issue_comment_reactions (
    comment_id uuid NOT NULL,
    account_id uuid NOT NULL,
    reaction   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, account_id, reaction),
    CONSTRAINT workspace_issue_comment_reactions_kind_check
        CHECK (reaction IN ('up', 'down', 'celebrate', 'thinking', 'eyes', 'heart')),
    CONSTRAINT workspace_issue_comment_reactions_comment_fkey
        FOREIGN KEY (comment_id) REFERENCES workspace_issue_comments (id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_comment_reactions_account_fkey
        FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted'));

-- +goose Down
ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged'));

DROP TABLE workspace_issue_comment_reactions;

DROP TABLE workspace_issue_comment_mentions;

DROP TABLE workspace_issue_comments;
