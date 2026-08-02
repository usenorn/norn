-- +goose Up
CREATE TABLE account_password_resets (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    used_at      timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX account_password_resets_token_key ON account_password_resets (token_hash);

CREATE UNIQUE INDEX account_password_resets_pending_key ON account_password_resets (account_id) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE account_password_resets;
