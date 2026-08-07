-- +goose Up
ALTER TABLE workspace_scm_connections
    ADD COLUMN allow_private_address boolean NOT NULL DEFAULT false,
    ADD COLUMN ca_certificate text NOT NULL DEFAULT '',
    ADD COLUMN capabilities text[] NOT NULL DEFAULT '{}';

ALTER TABLE workspace_scm_connections
    DROP CONSTRAINT workspace_scm_connections_provider_check,
    ADD CONSTRAINT workspace_scm_connections_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_scm_repositories
    DROP CONSTRAINT workspace_scm_repositories_provider_check,
    ADD CONSTRAINT workspace_scm_repositories_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_code_links
    DROP CONSTRAINT workspace_code_links_provider_check,
    ADD CONSTRAINT workspace_code_links_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_issue_mirrors
    DROP CONSTRAINT workspace_issue_mirrors_provider_check,
    ADD CONSTRAINT workspace_issue_mirrors_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_comment_mirrors
    DROP CONSTRAINT workspace_comment_mirrors_provider_check,
    ADD CONSTRAINT workspace_comment_mirrors_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_scm_identities
    DROP CONSTRAINT workspace_scm_identities_provider_check,
    ADD CONSTRAINT workspace_scm_identities_provider_check
        CHECK (provider IN ('github', 'gitlab', 'gitea'));

ALTER TABLE workspace_scm_repositories
    ADD COLUMN webhooks_disabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_scm_repositories
    DROP COLUMN webhooks_disabled;

DELETE FROM workspace_scm_identities WHERE provider = 'gitea';

ALTER TABLE workspace_scm_identities
    DROP CONSTRAINT workspace_scm_identities_provider_check,
    ADD CONSTRAINT workspace_scm_identities_provider_check
        CHECK (provider IN ('github', 'gitlab'));

DELETE FROM workspace_comment_mirrors WHERE provider = 'gitea';

ALTER TABLE workspace_comment_mirrors
    DROP CONSTRAINT workspace_comment_mirrors_provider_check,
    ADD CONSTRAINT workspace_comment_mirrors_provider_check
        CHECK (provider IN ('github', 'gitlab'));

DELETE FROM workspace_issue_mirrors WHERE provider = 'gitea';

ALTER TABLE workspace_issue_mirrors
    DROP CONSTRAINT workspace_issue_mirrors_provider_check,
    ADD CONSTRAINT workspace_issue_mirrors_provider_check
        CHECK (provider IN ('github', 'gitlab'));

DELETE FROM workspace_code_links WHERE provider = 'gitea';

ALTER TABLE workspace_code_links
    DROP CONSTRAINT workspace_code_links_provider_check,
    ADD CONSTRAINT workspace_code_links_provider_check
        CHECK (provider IN ('github', 'gitlab'));

DELETE FROM workspace_scm_repositories WHERE provider = 'gitea';

ALTER TABLE workspace_scm_repositories
    DROP CONSTRAINT workspace_scm_repositories_provider_check,
    ADD CONSTRAINT workspace_scm_repositories_provider_check
        CHECK (provider IN ('github', 'gitlab'));

DELETE FROM workspace_scm_connections WHERE provider = 'gitea';

ALTER TABLE workspace_scm_connections
    DROP CONSTRAINT workspace_scm_connections_provider_check,
    ADD CONSTRAINT workspace_scm_connections_provider_check
        CHECK (provider IN ('github', 'gitlab'));

ALTER TABLE workspace_scm_connections
    DROP COLUMN capabilities,
    DROP COLUMN ca_certificate,
    DROP COLUMN allow_private_address;
