-- +goose Up
CREATE TABLE account_password_history (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX account_password_history_account_created_idx
    ON account_password_history (account_id, created_at DESC);

-- +goose Down
DROP TABLE account_password_history;
