-- +goose Up
CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   uuid NOT NULL,
    workspace_id uuid NOT NULL,
    name         text NOT NULL,
    token_hash   bytea NOT NULL,
    scopes       text[] NOT NULL,
    expires_at   timestamptz,
    revoked_at   timestamptz,
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_tokens_scopes_check CHECK (cardinality(scopes) > 0),
    CONSTRAINT api_tokens_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX api_tokens_token_key ON api_tokens (token_hash);

CREATE UNIQUE INDEX api_tokens_live_name_key
    ON api_tokens (workspace_id, account_id, lower(name)) WHERE revoked_at IS NULL;

CREATE INDEX api_tokens_owner_idx ON api_tokens (workspace_id, account_id);

-- +goose Down
DROP TABLE api_tokens;
