-- +goose Up
CREATE TABLE account_email_changes (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    new_email    text NOT NULL,
    token_hash   bytea NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    confirmed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT account_email_changes_new_email_lower_check
        CHECK (new_email = lower(new_email))
);

CREATE UNIQUE INDEX account_email_changes_token_key ON account_email_changes (token_hash);

CREATE UNIQUE INDEX account_email_changes_pending_key ON account_email_changes (account_id) WHERE confirmed_at IS NULL;

-- +goose Down
DROP TABLE account_email_changes;
