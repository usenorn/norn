-- +goose Up
ALTER TABLE workspace_activity
    DROP CONSTRAINT IF EXISTS workspace_activity_actor_connection_id_fkey;

UPDATE workspace_activity
SET actor_connection_id = NULL
WHERE actor_connection_id IS NOT NULL;

DROP TABLE IF EXISTS mcp_tokens;

DROP TABLE IF EXISTS mcp_connection_grant_teams;

DROP TABLE IF EXISTS mcp_connection_grants;

DROP TABLE IF EXISTS mcp_connections;

DROP TABLE IF EXISTS mcp_clients;

-- +goose Down
CREATE TABLE mcp_clients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_clients_redirects_check CHECK (cardinality(redirect_uris) > 0)
);

CREATE TABLE mcp_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    client_id uuid NOT NULL REFERENCES mcp_clients (id) ON DELETE CASCADE,
    client_name text NOT NULL,
    scopes text[] NOT NULL,
    revoked_at timestamptz,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_connections_scopes_check CHECK (cardinality(scopes) > 0)
);

CREATE INDEX mcp_connections_account_idx ON mcp_connections (account_id);

CREATE TABLE mcp_connection_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id uuid NOT NULL REFERENCES mcp_connections (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    all_teams boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_connection_grants_connection_workspace_key UNIQUE (connection_id, workspace_id)
);

CREATE INDEX mcp_connection_grants_workspace_idx ON mcp_connection_grants (workspace_id);

CREATE TABLE mcp_connection_grant_teams (
    grant_id uuid NOT NULL REFERENCES mcp_connection_grants (id) ON DELETE CASCADE,
    team_id uuid NOT NULL REFERENCES workspace_teams (id) ON DELETE CASCADE,
    PRIMARY KEY (grant_id, team_id)
);

CREATE TABLE mcp_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id uuid NOT NULL REFERENCES mcp_connections (id) ON DELETE CASCADE,
    kind text NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mcp_tokens_kind_check CHECK (kind IN ('access', 'refresh'))
);

CREATE UNIQUE INDEX mcp_tokens_hash_key ON mcp_tokens (token_hash);

CREATE INDEX mcp_tokens_connection_idx ON mcp_tokens (connection_id);

CREATE INDEX mcp_tokens_expiring_idx ON mcp_tokens (expires_at);

ALTER TABLE workspace_activity
    ADD CONSTRAINT workspace_activity_actor_connection_id_fkey
    FOREIGN KEY (actor_connection_id) REFERENCES mcp_connections (id) ON DELETE SET NULL;
