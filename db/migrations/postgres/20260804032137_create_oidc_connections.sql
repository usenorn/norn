-- +goose Up
CREATE TABLE workspace_oidc_connections (
    workspace_id           uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    issuer                 text NOT NULL,
    discovered             boolean NOT NULL DEFAULT true,
    authorization_endpoint text NOT NULL,
    token_endpoint         text NOT NULL,
    jwks_uri               text NOT NULL,
    userinfo_endpoint      text NOT NULL DEFAULT '',
    client_id              text NOT NULL,
    client_secret_sealed   bytea NOT NULL,
    scopes                 text[] NOT NULL DEFAULT '{openid,email,profile}',
    groups_claim           text NOT NULL DEFAULT '',
    provisioning           boolean NOT NULL DEFAULT false,
    verified_at            timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_oidc_connections_issuer_check CHECK (issuer <> ''),
    CONSTRAINT workspace_oidc_connections_client_check CHECK (client_id <> ''),
    CONSTRAINT workspace_oidc_connections_secret_check CHECK (octet_length(client_secret_sealed) > 0),
    CONSTRAINT workspace_oidc_connections_endpoints_check
        CHECK (authorization_endpoint <> '' AND token_endpoint <> '' AND jwks_uri <> ''),
    CONSTRAINT workspace_oidc_connections_scopes_check CHECK ('openid' = ANY (scopes))
);

-- +goose Down
DROP TABLE workspace_oidc_connections;
