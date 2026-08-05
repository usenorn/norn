-- +goose Up
ALTER TABLE workspace_memberships
    ADD COLUMN deactivated_at timestamptz;

CREATE TABLE workspace_directory_connections (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    on_unknown text NOT NULL DEFAULT 'provision',
    on_absent text NOT NULL DEFAULT 'deactivate',
    admin_group text NOT NULL DEFAULT '',
    last_sync_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT directory_connections_on_unknown_check
        CHECK (on_unknown IN ('provision', 'ignore')),
    CONSTRAINT directory_connections_on_absent_check
        CHECK (on_absent IN ('deactivate', 'flag'))
);

CREATE UNIQUE INDEX directory_connections_token_idx
    ON workspace_directory_connections (token_hash);

CREATE TABLE directory_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace_directory_connections (workspace_id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    external_id text NOT NULL DEFAULT '',
    user_name text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT directory_users_account_key UNIQUE (workspace_id, account_id)
);

CREATE UNIQUE INDEX directory_users_name_idx
    ON directory_users (workspace_id, lower(user_name));

CREATE UNIQUE INDEX directory_users_external_idx
    ON directory_users (workspace_id, external_id)
    WHERE external_id <> '';

CREATE TABLE directory_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace_directory_connections (workspace_id) ON DELETE CASCADE,
    external_id text NOT NULL DEFAULT '',
    display_name text NOT NULL,
    team_id uuid REFERENCES workspace_teams (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX directory_groups_name_idx
    ON directory_groups (workspace_id, lower(display_name));

CREATE UNIQUE INDEX directory_groups_external_idx
    ON directory_groups (workspace_id, external_id)
    WHERE external_id <> '';

CREATE TABLE directory_group_members (
    group_id uuid NOT NULL REFERENCES directory_groups (id) ON DELETE CASCADE,
    directory_user_id uuid NOT NULL REFERENCES directory_users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, directory_user_id)
);

CREATE INDEX directory_group_members_user_idx
    ON directory_group_members (directory_user_id);

CREATE TABLE directory_sync_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    trigger text NOT NULL,
    outcome text NOT NULL,
    operation text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz NOT NULL DEFAULT now(),
    detail text NOT NULL DEFAULT '',
    CONSTRAINT directory_sync_runs_outcome_check
        CHECK (outcome IN ('succeeded', 'refused', 'failed'))
);

CREATE INDEX directory_sync_runs_workspace_idx
    ON directory_sync_runs (workspace_id, started_at DESC, id);

CREATE TABLE directory_sync_changes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES directory_sync_runs (id) ON DELETE CASCADE,
    subject text NOT NULL DEFAULT '',
    kind text NOT NULL,
    outcome text NOT NULL,
    detail jsonb,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT directory_sync_changes_outcome_check
        CHECK (outcome IN ('succeeded', 'refused', 'failed'))
);

CREATE INDEX directory_sync_changes_run_idx
    ON directory_sync_changes (run_id, recorded_at);

-- +goose Down
DROP TABLE IF EXISTS directory_sync_changes;

DROP TABLE IF EXISTS directory_sync_runs;

DROP TABLE IF EXISTS directory_group_members;

DROP TABLE IF EXISTS directory_groups;

DROP TABLE IF EXISTS directory_users;

DROP TABLE IF EXISTS workspace_directory_connections;

ALTER TABLE workspace_memberships DROP COLUMN deactivated_at;
