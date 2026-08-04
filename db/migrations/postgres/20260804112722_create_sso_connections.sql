-- +goose Up
CREATE TABLE workspace_sso_connections (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    protocol     text NOT NULL,
    provisioning boolean NOT NULL DEFAULT false,
    verified_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_sso_connections_protocol_check CHECK (protocol IN ('oidc', 'saml')),
    CONSTRAINT workspace_sso_connections_protocol_key UNIQUE (workspace_id, protocol)
);

INSERT INTO workspace_sso_connections (workspace_id, protocol, provisioning, verified_at, created_at, updated_at)
SELECT workspace_id, 'oidc', provisioning, verified_at, created_at, updated_at
FROM workspace_oidc_connections;

ALTER TABLE workspace_oidc_connections
    DROP COLUMN provisioning,
    DROP COLUMN verified_at,
    ADD COLUMN protocol text GENERATED ALWAYS AS ('oidc') STORED;

ALTER TABLE workspace_oidc_connections
    ADD CONSTRAINT workspace_oidc_connections_sso_fkey
        FOREIGN KEY (workspace_id, protocol)
        REFERENCES workspace_sso_connections (workspace_id, protocol)
        ON DELETE CASCADE;

CREATE TABLE workspace_saml_connections (
    workspace_id               uuid PRIMARY KEY,
    protocol                   text GENERATED ALWAYS AS ('saml') STORED,
    entity_id                  text NOT NULL,
    sso_url                    text NOT NULL,
    slo_url                    text NOT NULL DEFAULT '',
    metadata_url               text NOT NULL DEFAULT '',
    idp_certificates           text[] NOT NULL,
    idp_certificate_expires_at timestamptz NOT NULL,
    sp_entity_id               text NOT NULL,
    sp_private_key_sealed      bytea NOT NULL,
    sp_certificate             text NOT NULL,
    allow_idp_initiated        boolean NOT NULL DEFAULT false,
    email_attribute            text NOT NULL DEFAULT '',
    name_attribute             text NOT NULL DEFAULT '',
    groups_attribute           text NOT NULL DEFAULT '',
    expiry_notice_days         integer,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_saml_connections_sso_fkey
        FOREIGN KEY (workspace_id, protocol)
        REFERENCES workspace_sso_connections (workspace_id, protocol)
        ON DELETE CASCADE,
    CONSTRAINT workspace_saml_connections_entity_check CHECK (entity_id <> ''),
    CONSTRAINT workspace_saml_connections_sso_url_check CHECK (sso_url <> ''),
    CONSTRAINT workspace_saml_connections_sp_entity_check CHECK (sp_entity_id <> ''),
    CONSTRAINT workspace_saml_connections_certificates_check
        CHECK (cardinality(idp_certificates) > 0),
    CONSTRAINT workspace_saml_connections_key_check
        CHECK (octet_length(sp_private_key_sealed) > 0),
    CONSTRAINT workspace_saml_connections_sp_certificate_check CHECK (sp_certificate <> ''),
    CONSTRAINT workspace_saml_connections_notice_check
        CHECK (expiry_notice_days IS NULL OR expiry_notice_days > 0)
);

-- +goose Down
DROP TABLE workspace_saml_connections;

ALTER TABLE workspace_oidc_connections
    DROP CONSTRAINT workspace_oidc_connections_sso_fkey;

ALTER TABLE workspace_oidc_connections
    DROP COLUMN protocol,
    ADD COLUMN provisioning boolean NOT NULL DEFAULT false,
    ADD COLUMN verified_at timestamptz;

UPDATE workspace_oidc_connections o
SET provisioning = s.provisioning, verified_at = s.verified_at
FROM workspace_sso_connections s
WHERE s.workspace_id = o.workspace_id;

DROP TABLE workspace_sso_connections;
