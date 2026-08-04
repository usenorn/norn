-- +goose Up
CREATE TABLE account_sign_ups (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    display_name  text NOT NULL,
    timezone      text NOT NULL,
    password_hash text NOT NULL,
    token_hash    bytea NOT NULL,
    requested_at  timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    confirmed_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT account_sign_ups_email_lower_check CHECK (email = lower(email))
);

CREATE UNIQUE INDEX account_sign_ups_token_key ON account_sign_ups (token_hash);

CREATE UNIQUE INDEX account_sign_ups_pending_key
    ON account_sign_ups (email) WHERE confirmed_at IS NULL;

-- +goose Down
DROP TABLE account_sign_ups;
