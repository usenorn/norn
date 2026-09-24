-- +goose Up
CREATE TABLE workspace_agent_skills (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    agent_id              uuid REFERENCES workspace_agents (id) ON DELETE CASCADE,
    name                  text NOT NULL,
    description           text NOT NULL,
    source                text NOT NULL,
    origin                text NOT NULL DEFAULT '',
    path                  text NOT NULL DEFAULT '',
    ref                   text NOT NULL DEFAULT '',
    revision              text NOT NULL DEFAULT '',
    instructions          text NOT NULL,
    content_hash          text NOT NULL,
    object_key            text NOT NULL,
    size_bytes            bigint NOT NULL,
    file_count            integer NOT NULL,
    created_by_account_id uuid NOT NULL REFERENCES accounts (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agent_skills_source_check CHECK (source IN ('manual', 'github')),
    CONSTRAINT workspace_agent_skills_origin_check CHECK ((source = 'github') = (origin <> '')),
    CONSTRAINT workspace_agent_skills_size_check CHECK (size_bytes > 0 AND file_count > 0)
);

CREATE UNIQUE INDEX workspace_agent_skills_agent_name_key
    ON workspace_agent_skills (agent_id, lower(name)) WHERE agent_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_agent_skills_library_name_key
    ON workspace_agent_skills (workspace_id, lower(name)) WHERE agent_id IS NULL;

CREATE TABLE workspace_agent_mcp_servers (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    agent_id              uuid REFERENCES workspace_agents (id) ON DELETE CASCADE,
    name                  text NOT NULL,
    transport             text NOT NULL,
    command               text NOT NULL DEFAULT '',
    args                  text[] NOT NULL DEFAULT '{}',
    url                   text NOT NULL DEFAULT '',
    auth                  text NOT NULL DEFAULT 'none',
    env_keys              text[] NOT NULL DEFAULT '{}',
    header_keys           text[] NOT NULL DEFAULT '{}',
    oauth_client_id       text NOT NULL DEFAULT '',
    secrets_sealed        bytea,
    registry_name         text NOT NULL DEFAULT '',
    registry_version      text NOT NULL DEFAULT '',
    created_by_account_id uuid NOT NULL REFERENCES accounts (id),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agent_mcp_servers_transport_check CHECK (transport IN ('stdio', 'http', 'sse')),
    CONSTRAINT workspace_agent_mcp_servers_auth_check CHECK (auth IN ('none', 'headers', 'oauth')),
    CONSTRAINT workspace_agent_mcp_servers_shape_check CHECK (
        (transport = 'stdio' AND command <> '' AND url = '' AND auth = 'none')
        OR (transport <> 'stdio' AND url <> '' AND command = '')
    ),
    CONSTRAINT workspace_agent_mcp_servers_headers_check CHECK (
        transport <> 'stdio' OR cardinality(header_keys) = 0
    )
);

CREATE UNIQUE INDEX workspace_agent_mcp_servers_agent_name_key
    ON workspace_agent_mcp_servers (agent_id, lower(name)) WHERE agent_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_agent_mcp_servers_library_name_key
    ON workspace_agent_mcp_servers (workspace_id, lower(name)) WHERE agent_id IS NULL;

CREATE TABLE workspace_agent_mcp_connections (
    server_id               uuid PRIMARY KEY REFERENCES workspace_agent_mcp_servers (id) ON DELETE CASCADE,
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    status                  text NOT NULL,
    issuer                  text NOT NULL,
    token_endpoint          text NOT NULL,
    client_id               text NOT NULL,
    client_secret_sealed    bytea,
    scopes                  text[] NOT NULL DEFAULT '{}',
    access_token_sealed     bytea NOT NULL,
    refresh_token_sealed    bytea,
    expires_at              timestamptz,
    failure                 text NOT NULL DEFAULT '',
    connected_by_account_id uuid NOT NULL REFERENCES accounts (id),
    connected_at            timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agent_mcp_connections_status_check CHECK (status IN ('connected', 'expired', 'failed')),
    CONSTRAINT workspace_agent_mcp_connections_failure_check CHECK ((status = 'failed') = (failure <> ''))
);

CREATE TABLE workspace_agent_skill_attachments (
    agent_id               uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    skill_id               uuid NOT NULL REFERENCES workspace_agent_skills (id) ON DELETE CASCADE,
    workspace_id           uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    attached_by_account_id uuid NOT NULL REFERENCES accounts (id),
    attached_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, skill_id)
);

CREATE INDEX workspace_agent_skill_attachments_skill_idx ON workspace_agent_skill_attachments (skill_id);

CREATE TABLE workspace_agent_mcp_server_attachments (
    agent_id               uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    server_id              uuid NOT NULL REFERENCES workspace_agent_mcp_servers (id) ON DELETE CASCADE,
    workspace_id           uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    attached_by_account_id uuid NOT NULL REFERENCES accounts (id),
    attached_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, server_id)
);

CREATE INDEX workspace_agent_mcp_server_attachments_server_idx
    ON workspace_agent_mcp_server_attachments (server_id);

-- +goose Down
DROP TABLE workspace_agent_mcp_server_attachments;
DROP TABLE workspace_agent_skill_attachments;
DROP TABLE workspace_agent_mcp_connections;
DROP TABLE workspace_agent_mcp_servers;
DROP TABLE workspace_agent_skills;
