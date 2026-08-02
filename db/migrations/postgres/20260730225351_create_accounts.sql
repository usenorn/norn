-- +goose Up
CREATE TABLE accounts (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status            text NOT NULL,
    email             text,
    display_name      text,
    avatar_object_key text,
    timezone          text,
    password_hash     text,
    deactivated_at    timestamptz,
    deleted_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounts_status_check
        CHECK (status IN ('active', 'deactivated', 'deleted')),
    CONSTRAINT accounts_active_identity_check
        CHECK (status <> 'active' OR (email IS NOT NULL AND display_name IS NOT NULL AND timezone IS NOT NULL AND deactivated_at IS NULL AND deleted_at IS NULL)),
    CONSTRAINT accounts_deactivated_identity_check
        CHECK (status <> 'deactivated' OR (email IS NOT NULL AND display_name IS NOT NULL AND timezone IS NOT NULL AND deactivated_at IS NOT NULL AND deleted_at IS NULL)),
    CONSTRAINT accounts_deleted_identity_check
        CHECK (status <> 'deleted' OR (email IS NULL AND display_name IS NULL AND timezone IS NULL AND password_hash IS NULL AND avatar_object_key IS NULL AND deleted_at IS NOT NULL))
);

CREATE UNIQUE INDEX accounts_email_lower_key ON accounts (lower(email)) WHERE email IS NOT NULL;

-- +goose Down
DROP TABLE accounts;
