-- +goose Up
DROP TABLE IF EXISTS api_tokens;

CREATE TABLE api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name text NOT NULL,
    token_hash bytea NOT NULL,
    scopes text[] NOT NULL,
    expires_at timestamptz,
    revoked_at timestamptz,
    last_used_at timestamptz,
    expiry_notice_days integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_tokens_scopes_check CHECK (cardinality(scopes) > 0),
    CONSTRAINT api_tokens_notice_check CHECK (expiry_notice_days IS NULL OR expiry_notice_days > 0)
);

CREATE UNIQUE INDEX api_tokens_token_key ON api_tokens (token_hash);

CREATE UNIQUE INDEX api_tokens_live_name_key
    ON api_tokens (account_id, lower(name)) WHERE revoked_at IS NULL;

CREATE INDEX api_tokens_expiring_idx
    ON api_tokens (expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL;

CREATE TABLE api_token_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id uuid NOT NULL REFERENCES api_tokens (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    all_teams boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_token_grants_token_workspace_key UNIQUE (token_id, workspace_id)
);

CREATE INDEX api_token_grants_workspace_idx ON api_token_grants (workspace_id);

CREATE TABLE api_token_grant_teams (
    grant_id uuid NOT NULL REFERENCES api_token_grants (id) ON DELETE CASCADE,
    team_id uuid NOT NULL REFERENCES workspace_teams (id) ON DELETE CASCADE,
    PRIMARY KEY (grant_id, team_id)
);

-- +goose Down
DROP TABLE IF EXISTS api_token_grant_teams;

DROP TABLE IF EXISTS api_token_grants;

DROP TABLE IF EXISTS api_tokens;

CREATE TABLE api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    name text NOT NULL,
    token_hash bytea NOT NULL,
    scopes text[] NOT NULL,
    expires_at timestamptz,
    revoked_at timestamptz,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_tokens_scopes_check CHECK (cardinality(scopes) > 0),
    CONSTRAINT api_tokens_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX api_tokens_token_key ON api_tokens (token_hash);

CREATE UNIQUE INDEX api_tokens_live_name_key
    ON api_tokens (workspace_id, account_id, lower(name)) WHERE revoked_at IS NULL;

CREATE INDEX api_tokens_owner_idx ON api_tokens (workspace_id, account_id);
