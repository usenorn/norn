-- +goose Up
CREATE TABLE workspace_domains (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    domain       text NOT NULL,
    token        text NOT NULL,
    verified_at  timestamptz,
    checked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_domains_workspace_domain_key UNIQUE (workspace_id, domain),
    CONSTRAINT workspace_domains_domain_check CHECK (domain = lower(domain) AND length(domain) BETWEEN 3 AND 253),
    CONSTRAINT workspace_domains_token_check CHECK (length(token) > 0)
);

CREATE INDEX workspace_domains_verified_domain_idx ON workspace_domains (domain) WHERE verified_at IS NOT NULL;

-- +goose Down
DROP TABLE workspace_domains;
