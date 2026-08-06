-- +goose Up
ALTER TABLE workspace_sso_identities ADD COLUMN issuer text NOT NULL DEFAULT '';

UPDATE workspace_sso_identities i
SET issuer = o.issuer
FROM workspace_oidc_connections o
WHERE o.workspace_id = i.workspace_id;

UPDATE workspace_sso_identities i
SET issuer = s.entity_id
FROM workspace_saml_connections s
WHERE s.workspace_id = i.workspace_id AND i.issuer = '';

DELETE FROM workspace_sso_identities WHERE issuer = '';

ALTER TABLE workspace_sso_identities ALTER COLUMN issuer DROP DEFAULT;

ALTER TABLE workspace_sso_identities
    ADD CONSTRAINT workspace_sso_identities_issuer_check CHECK (issuer <> ''),
    DROP CONSTRAINT workspace_sso_identities_subject_key,
    ADD CONSTRAINT workspace_sso_identities_subject_key UNIQUE (workspace_id, issuer, subject);

-- +goose Down
ALTER TABLE workspace_sso_identities DROP COLUMN issuer;

ALTER TABLE workspace_sso_identities
    ADD CONSTRAINT workspace_sso_identities_subject_key UNIQUE (workspace_id, subject);
