-- +goose Up
CREATE TABLE workspace_teams (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    key          text NOT NULL,
    name         text NOT NULL,
    status       text NOT NULL DEFAULT 'active',
    visibility   text NOT NULL DEFAULT 'public',
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_teams_status_check
        CHECK (status IN ('active', 'archived')),
    CONSTRAINT workspace_teams_visibility_check
        CHECK (visibility IN ('public', 'private')),
    CONSTRAINT workspace_teams_archived_check
        CHECK (status <> 'archived' OR archived_at IS NOT NULL),
    CONSTRAINT workspace_teams_key_upper_check
        CHECK (key = upper(key))
);

CREATE UNIQUE INDEX workspace_teams_workspace_key_key ON workspace_teams (workspace_id, key);

CREATE UNIQUE INDEX workspace_teams_id_workspace_key ON workspace_teams (id, workspace_id);

CREATE INDEX workspace_teams_workspace_status_idx ON workspace_teams (workspace_id, status);

CREATE TABLE workspace_team_members (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    team_id      uuid NOT NULL,
    account_id   uuid NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_members_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_team_members_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_team_members_team_account_key
    ON workspace_team_members (team_id, account_id);

CREATE INDEX workspace_team_members_workspace_account_idx
    ON workspace_team_members (workspace_id, account_id);

ALTER TABLE workspaces
    ADD COLUMN default_team_id uuid REFERENCES workspace_teams (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE workspaces
    DROP COLUMN default_team_id;

DROP TABLE workspace_team_members;

DROP TABLE workspace_teams;
