-- +goose Up
CREATE TABLE workspace_scm_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    provider text NOT NULL,
    login text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_scm_identities_provider_check CHECK (provider IN ('github', 'gitlab')),
    CONSTRAINT workspace_scm_identities_login_check CHECK (login <> '')
);

CREATE UNIQUE INDEX workspace_scm_identities_login_key
    ON workspace_scm_identities (workspace_id, provider, lower(login));

CREATE UNIQUE INDEX workspace_scm_identities_account_key
    ON workspace_scm_identities (workspace_id, provider, account_id);

CREATE TABLE workspace_issue_mirror_conflicts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    mirror_id uuid NOT NULL REFERENCES workspace_issue_mirrors (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    field text NOT NULL,
    winner text NOT NULL,
    discarded text NOT NULL,
    kept text NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_mirror_conflicts_winner_check
        CHECK (winner IN ('norn', 'source'))
);

CREATE INDEX workspace_issue_mirror_conflicts_issue_idx
    ON workspace_issue_mirror_conflicts (issue_id, occurred_at DESC, id);

ALTER TABLE workspace_scm_repositories
    ADD COLUMN sync_direction text NOT NULL DEFAULT 'both',
    ADD CONSTRAINT workspace_scm_repositories_direction_check
        CHECK (sync_direction IN ('inbound', 'outbound', 'both'));

-- +goose Down
ALTER TABLE workspace_scm_repositories
    DROP CONSTRAINT workspace_scm_repositories_direction_check,
    DROP COLUMN sync_direction;

DROP TABLE IF EXISTS workspace_issue_mirror_conflicts;

DROP TABLE IF EXISTS workspace_scm_identities;
