-- +goose Up
ALTER TABLE accounts ADD COLUMN kind text NOT NULL DEFAULT 'person';

ALTER TABLE accounts
    ADD CONSTRAINT accounts_kind_check CHECK (kind IN ('person', 'agent')),
    ADD CONSTRAINT accounts_agent_credential_check
        CHECK (kind <> 'agent' OR password_hash IS NULL);

ALTER TABLE accounts DROP CONSTRAINT accounts_active_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_active_identity_check
        CHECK (status <> 'active' OR (
            (kind = 'agent' OR email IS NOT NULL)
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NULL
            AND deleted_at IS NULL));

-- +goose Down
ALTER TABLE accounts DROP CONSTRAINT accounts_active_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_active_identity_check
        CHECK (status <> 'active' OR (
            email IS NOT NULL
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NULL
            AND deleted_at IS NULL));

ALTER TABLE accounts
    DROP CONSTRAINT accounts_agent_credential_check,
    DROP CONSTRAINT accounts_kind_check,
    DROP COLUMN kind;
