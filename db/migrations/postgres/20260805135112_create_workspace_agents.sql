-- +goose Up
CREATE TABLE workspace_agents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id uuid NOT NULL UNIQUE REFERENCES accounts (id) ON DELETE CASCADE,
    owner_account_id uuid NOT NULL REFERENCES accounts (id),
    name text NOT NULL,
    status text NOT NULL DEFAULT 'active',
    action_limit integer,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agents_status_check CHECK (status IN ('active', 'disabled')),
    CONSTRAINT workspace_agents_limit_check CHECK (action_limit IS NULL OR action_limit > 0),
    CONSTRAINT workspace_agents_disabled_check
        CHECK ((status = 'disabled') = (disabled_at IS NOT NULL))
);

CREATE UNIQUE INDEX workspace_agents_live_name_key
    ON workspace_agents (workspace_id, lower(name)) WHERE status <> 'disabled';

CREATE INDEX workspace_agents_owner_idx ON workspace_agents (workspace_id, owner_account_id);

CREATE TABLE workspace_team_agent_settings (
    team_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    hold_comments boolean NOT NULL DEFAULT false,
    hold_state_changes boolean NOT NULL DEFAULT false,
    hold_issue_edits boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_agent_settings_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_team_agent_settings_workspace_idx
    ON workspace_team_agent_settings (workspace_id);

CREATE TABLE workspace_agent_proposals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    team_id uuid NOT NULL,
    action text NOT NULL,
    change jsonb NOT NULL,
    status text NOT NULL DEFAULT 'pending',
    decided_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    decided_at timestamptz,
    failure text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit')),
    CONSTRAINT workspace_agent_proposals_status_check
        CHECK (status IN ('pending', 'rejected', 'applied', 'failed')),
    CONSTRAINT workspace_agent_proposals_decided_check
        CHECK ((status = 'pending') = (decided_at IS NULL))
);

CREATE INDEX workspace_agent_proposals_waiting_idx
    ON workspace_agent_proposals (workspace_id, created_at, id) WHERE status = 'pending';

CREATE INDEX workspace_agent_proposals_agent_idx
    ON workspace_agent_proposals (agent_id, created_at DESC);

CREATE INDEX workspace_activity_actor_idx
    ON workspace_activity (workspace_id, actor_account_id, created_at DESC, id)
    WHERE actor_account_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS workspace_activity_actor_idx;

DROP TABLE IF EXISTS workspace_agent_proposals;

DROP TABLE IF EXISTS workspace_team_agent_settings;

DROP TABLE IF EXISTS workspace_agents;
