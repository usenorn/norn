-- +goose Up
CREATE TABLE preview_gateways (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL,
    secret_hash  bytea NOT NULL,
    status       text NOT NULL DEFAULT 'active',
    last_seen_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT preview_gateways_name_check CHECK (name <> ''),
    CONSTRAINT preview_gateways_status_check CHECK (status IN ('active', 'revoked'))
);

CREATE UNIQUE INDEX preview_gateways_name_key ON preview_gateways (lower(name));

CREATE UNIQUE INDEX preview_gateways_secret_key ON preview_gateways (secret_hash);

CREATE TABLE workspace_execution_previews (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    service      text NOT NULL,
    path         text NOT NULL DEFAULT '',
    mode         text NOT NULL DEFAULT 'subdomain',
    host         text NOT NULL DEFAULT '',
    state        text NOT NULL,
    opened_at    timestamptz NOT NULL,
    closed_at    timestamptz,
    reported_at  timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_previews_name_check CHECK (name <> ''),
    CONSTRAINT workspace_execution_previews_service_check CHECK (service <> ''),
    CONSTRAINT workspace_execution_previews_mode_check CHECK (mode IN ('subdomain', 'path')),
    CONSTRAINT workspace_execution_previews_state_check CHECK (state IN ('open', 'closed'))
);

CREATE UNIQUE INDEX workspace_execution_previews_name_key
    ON workspace_execution_previews (execution_id, name);

CREATE UNIQUE INDEX workspace_execution_previews_host_key
    ON workspace_execution_previews (host) WHERE host <> '';

CREATE INDEX workspace_execution_previews_execution_idx
    ON workspace_execution_previews (execution_id, name, id);

CREATE TABLE workspace_execution_preview_links (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    preview_id    uuid NOT NULL REFERENCES workspace_execution_previews (id) ON DELETE CASCADE,
    execution_id  text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    token_hash    bytea NOT NULL,
    passcode_hash text NOT NULL DEFAULT '',
    created_by    uuid REFERENCES accounts (id) ON DELETE SET NULL,
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz,
    last_used_at  timestamptz,
    uses          integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_preview_links_uses_check CHECK (uses >= 0)
);

CREATE UNIQUE INDEX workspace_execution_preview_links_token_key
    ON workspace_execution_preview_links (token_hash);

CREATE INDEX workspace_execution_preview_links_preview_idx
    ON workspace_execution_preview_links (preview_id, created_at, id);

-- +goose Down
DROP TABLE workspace_execution_preview_links;

DROP TABLE workspace_execution_previews;

DROP TABLE preview_gateways;
