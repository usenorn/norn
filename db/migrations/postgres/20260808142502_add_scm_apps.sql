-- +goose Up
CREATE TABLE scm_apps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    base_url text NOT NULL DEFAULT '',
    slug text NOT NULL DEFAULT '',
    external_app_id text NOT NULL,
    client_id text NOT NULL DEFAULT '',
    client_secret_sealed bytea,
    private_key_sealed bytea NOT NULL,
    webhook_secret_sealed bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scm_apps_provider_check CHECK (provider IN ('github')),
    CONSTRAINT scm_apps_external_check CHECK (external_app_id <> ''),
    CONSTRAINT scm_apps_private_key_check CHECK (octet_length(private_key_sealed) > 0),
    CONSTRAINT scm_apps_webhook_secret_check CHECK (octet_length(webhook_secret_sealed) > 0)
);

CREATE UNIQUE INDEX scm_apps_endpoint_key ON scm_apps (provider, base_url);

ALTER TABLE workspace_scm_connections
    ADD COLUMN auth_kind text NOT NULL DEFAULT 'token',
    ADD COLUMN app_id uuid REFERENCES scm_apps (id) ON DELETE RESTRICT,
    ADD COLUMN installation_id text NOT NULL DEFAULT '',
    ADD COLUMN account_login text NOT NULL DEFAULT '',
    ADD CONSTRAINT workspace_scm_connections_auth_kind_check
        CHECK (auth_kind IN ('token', 'app'));

ALTER TABLE workspace_scm_connections
    DROP CONSTRAINT workspace_scm_connections_token_check,
    ALTER COLUMN token_sealed DROP NOT NULL,
    ADD CONSTRAINT workspace_scm_connections_credential_check
        CHECK (
            (auth_kind = 'token' AND octet_length(token_sealed) > 0)
            OR (auth_kind = 'app' AND app_id IS NOT NULL AND installation_id <> '')
        );

CREATE UNIQUE INDEX workspace_scm_connections_installation_key
    ON workspace_scm_connections (app_id, installation_id)
    WHERE auth_kind = 'app';

ALTER TABLE workspace_scm_repositories
    DROP CONSTRAINT workspace_scm_repositories_secret_check,
    ALTER COLUMN webhook_secret_sealed DROP NOT NULL;

-- +goose Down
DELETE FROM workspace_scm_repositories
WHERE connection_id IN (SELECT id FROM workspace_scm_connections WHERE auth_kind = 'app');

DELETE FROM workspace_scm_connections WHERE auth_kind = 'app';

UPDATE workspace_scm_repositories
SET webhook_secret_sealed = '\x00'::bytea
WHERE webhook_secret_sealed IS NULL;

ALTER TABLE workspace_scm_repositories
    ALTER COLUMN webhook_secret_sealed SET NOT NULL,
    ADD CONSTRAINT workspace_scm_repositories_secret_check
        CHECK (octet_length(webhook_secret_sealed) > 0);

DROP INDEX IF EXISTS workspace_scm_connections_installation_key;

ALTER TABLE workspace_scm_connections
    DROP CONSTRAINT workspace_scm_connections_credential_check,
    ALTER COLUMN token_sealed SET NOT NULL,
    ADD CONSTRAINT workspace_scm_connections_token_check
        CHECK (octet_length(token_sealed) > 0);

ALTER TABLE workspace_scm_connections
    DROP CONSTRAINT workspace_scm_connections_auth_kind_check,
    DROP COLUMN account_login,
    DROP COLUMN installation_id,
    DROP COLUMN app_id,
    DROP COLUMN auth_kind;

DROP TABLE IF EXISTS scm_apps;
