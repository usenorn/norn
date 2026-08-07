-- +goose Up
-- Who somebody is on a forge. Without this the integration can read a login and do nothing
-- with it: matching on a display name is how work lands on a stranger with a similar name.
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

-- One login belongs to one person here, and one person has one login per forge. Both
-- directions have to be unique or a lookup either way has no single answer.
CREATE UNIQUE INDEX workspace_scm_identities_login_key
    ON workspace_scm_identities (workspace_id, provider, lower(login));

CREATE UNIQUE INDEX workspace_scm_identities_account_key
    ON workspace_scm_identities (workspace_id, provider, account_id);

-- The edit that lost. Arbitration has to pick one side, and a rule that silently discards
-- the other is the failure this whole feature exists to avoid; the value is kept so the
-- person who wrote it can get it back.
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

-- Direction belongs to the repository, not to each pairing. A repository Norn only reads
-- from and one it only writes to are both ordinary arrangements, and asking per issue would
-- put the decision in the wrong place.
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
