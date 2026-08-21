-- +goose Up
CREATE TABLE workspace_runners (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    name text NOT NULL,
    hostname text NOT NULL,
    os text NOT NULL,
    arch text NOT NULL,
    runner_version text NOT NULL,
    all_teams boolean NOT NULL DEFAULT false,
    team_ids uuid[] NOT NULL DEFAULT '{}',
    scopes text[] NOT NULL,
    public_key bytea NOT NULL,
    refresh_hash bytea NOT NULL,
    status text NOT NULL DEFAULT 'active',
    enrolled_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz,
    revoked_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_runners_status_check CHECK (status IN ('active', 'revoked')),
    CONSTRAINT workspace_runners_revoked_check
        CHECK ((status = 'revoked') = (revoked_at IS NOT NULL)),
    CONSTRAINT workspace_runners_name_check CHECK (name <> ''),
    CONSTRAINT workspace_runners_hostname_check CHECK (hostname <> ''),
    CONSTRAINT workspace_runners_scopes_check CHECK (cardinality(scopes) > 0),
    CONSTRAINT workspace_runners_teams_check
        CHECK (all_teams OR cardinality(team_ids) > 0),
    CONSTRAINT workspace_runners_public_key_check CHECK (octet_length(public_key) = 32),
    CONSTRAINT workspace_runners_refresh_hash_check CHECK (octet_length(refresh_hash) = 32)
);

CREATE UNIQUE INDEX workspace_runners_refresh_hash_key
    ON workspace_runners (refresh_hash);

CREATE UNIQUE INDEX workspace_runners_live_name_key
    ON workspace_runners (agent_id, lower(name)) WHERE status <> 'revoked';

CREATE INDEX workspace_runners_workspace_idx
    ON workspace_runners (workspace_id, enrolled_at DESC, id);

CREATE INDEX workspace_runners_agent_idx
    ON workspace_runners (agent_id) WHERE status <> 'revoked';

-- +goose Down
DROP TABLE workspace_runners;
