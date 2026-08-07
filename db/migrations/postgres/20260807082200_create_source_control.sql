-- +goose Up
CREATE TABLE workspace_scm_connections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    team_id uuid REFERENCES workspace_teams (id) ON DELETE CASCADE,
    provider text NOT NULL,
    base_url text NOT NULL DEFAULT '',
    repository text NOT NULL,
    external_repository_id text NOT NULL DEFAULT '',
    token_sealed bytea NOT NULL,
    token_hint text NOT NULL DEFAULT '',
    identity_login text NOT NULL DEFAULT '',
    webhook_secret_sealed bytea NOT NULL,
    external_hook_id text NOT NULL DEFAULT '',
    integration_account_id uuid NOT NULL REFERENCES accounts (id),
    owner_account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    owner_actor_kind text NOT NULL DEFAULT 'user',
    owner_auth_method text NOT NULL DEFAULT '',
    mirror_label text NOT NULL DEFAULT 'norn',
    status text NOT NULL DEFAULT 'connected',
    broken_reason text NOT NULL DEFAULT '',
    broken_detail text NOT NULL DEFAULT '',
    broken_at timestamptz,
    verified_at timestamptz,
    last_seen_at timestamptz,
    reconcile_cursor text NOT NULL DEFAULT '',
    reconciled_at timestamptz,
    reconcile_after timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_scm_connections_provider_check
        CHECK (provider IN ('github', 'gitlab')),
    CONSTRAINT workspace_scm_connections_status_check
        CHECK (status IN ('connected', 'broken')),
    CONSTRAINT workspace_scm_connections_broken_check
        CHECK (status <> 'broken' OR (broken_reason <> '' AND broken_at IS NOT NULL)),
    CONSTRAINT workspace_scm_connections_broken_reason_check
        CHECK (broken_reason IN ('', 'credentials_rejected', 'repository_unreachable',
                                 'hook_missing')),
    CONSTRAINT workspace_scm_connections_token_check
        CHECK (octet_length(token_sealed) > 0),
    CONSTRAINT workspace_scm_connections_secret_check
        CHECK (octet_length(webhook_secret_sealed) > 0),
    CONSTRAINT workspace_scm_connections_repository_check CHECK (repository <> ''),
    CONSTRAINT workspace_scm_connections_mirror_label_check CHECK (mirror_label <> '')
);

CREATE UNIQUE INDEX workspace_scm_connections_repository_key
    ON workspace_scm_connections (workspace_id, provider, base_url, repository);

CREATE INDEX workspace_scm_connections_workspace_idx
    ON workspace_scm_connections (workspace_id, created_at DESC, id);

CREATE INDEX workspace_scm_connections_due_idx
    ON workspace_scm_connections (reconciled_at NULLS FIRST, id)
    WHERE status = 'connected';

CREATE TABLE workspace_team_scm_settings (
    team_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    advance_on_merge boolean NOT NULL DEFAULT false,
    merged_state_id uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_scm_settings_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_team_scm_settings_workspace_idx
    ON workspace_team_scm_settings (workspace_id);

CREATE TABLE workspace_code_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    connection_id uuid REFERENCES workspace_scm_connections (id) ON DELETE SET NULL,
    provider text NOT NULL,
    repository text NOT NULL,
    kind text NOT NULL,
    external_id text NOT NULL,
    number integer NOT NULL DEFAULT 0,
    title text NOT NULL DEFAULT '',
    url text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'open',
    author text NOT NULL DEFAULT '',
    detected_in text NOT NULL DEFAULT '',
    advanced_issue boolean NOT NULL DEFAULT false,
    source_updated_at timestamptz,
    merged_at timestamptz,
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_code_links_provider_check CHECK (provider IN ('github', 'gitlab')),
    CONSTRAINT workspace_code_links_kind_check CHECK (kind IN ('branch', 'commit', 'change')),
    CONSTRAINT workspace_code_links_state_check
        CHECK (state IN ('open', 'draft', 'in_review', 'merged', 'closed')),
    CONSTRAINT workspace_code_links_external_check CHECK (external_id <> '')
);

CREATE UNIQUE INDEX workspace_code_links_external_key
    ON workspace_code_links (issue_id, provider, repository, kind, external_id);

CREATE INDEX workspace_code_links_issue_idx
    ON workspace_code_links (issue_id, created_at DESC, id);

CREATE INDEX workspace_code_links_connection_idx
    ON workspace_code_links (connection_id, state);

CREATE TABLE workspace_issue_mirrors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    connection_id uuid REFERENCES workspace_scm_connections (id) ON DELETE SET NULL,
    provider text NOT NULL,
    repository text NOT NULL,
    external_id text NOT NULL,
    external_number integer NOT NULL DEFAULT 0,
    url text NOT NULL DEFAULT '',
    origin text NOT NULL,
    title_hash text NOT NULL DEFAULT '',
    body_hash text NOT NULL DEFAULT '',
    state_hash text NOT NULL DEFAULT '',
    synced_version integer NOT NULL DEFAULT 0,
    source_updated_at timestamptz,
    pulled_at timestamptz,
    pushed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_mirrors_provider_check CHECK (provider IN ('github', 'gitlab')),
    CONSTRAINT workspace_issue_mirrors_origin_check CHECK (origin IN ('platform', 'norn')),
    CONSTRAINT workspace_issue_mirrors_external_check CHECK (external_id <> '')
);

CREATE UNIQUE INDEX workspace_issue_mirrors_issue_key ON workspace_issue_mirrors (issue_id);

CREATE UNIQUE INDEX workspace_issue_mirrors_external_key
    ON workspace_issue_mirrors (workspace_id, provider, repository, external_id);

CREATE TABLE workspace_comment_mirrors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    comment_id uuid NOT NULL REFERENCES workspace_issue_comments (id) ON DELETE CASCADE,
    connection_id uuid REFERENCES workspace_scm_connections (id) ON DELETE SET NULL,
    provider text NOT NULL,
    repository text NOT NULL,
    external_id text NOT NULL,
    external_author text NOT NULL DEFAULT '',
    origin text NOT NULL,
    body_hash text NOT NULL DEFAULT '',
    source_updated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_comment_mirrors_provider_check CHECK (provider IN ('github', 'gitlab')),
    CONSTRAINT workspace_comment_mirrors_origin_check CHECK (origin IN ('platform', 'norn')),
    CONSTRAINT workspace_comment_mirrors_external_check CHECK (external_id <> '')
);

CREATE UNIQUE INDEX workspace_comment_mirrors_comment_key
    ON workspace_comment_mirrors (comment_id);

CREATE UNIQUE INDEX workspace_comment_mirrors_external_key
    ON workspace_comment_mirrors (workspace_id, provider, repository, external_id);

CREATE INDEX workspace_comment_mirrors_issue_idx ON workspace_comment_mirrors (issue_id);

CREATE TABLE workspace_scm_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id uuid NOT NULL REFERENCES workspace_scm_connections (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    external_delivery_id text NOT NULL,
    event text NOT NULL,
    payload jsonb NOT NULL,
    attempt integer NOT NULL DEFAULT 0,
    retry_after timestamptz,
    failure text NOT NULL DEFAULT '',
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    CONSTRAINT workspace_scm_deliveries_attempt_check CHECK (attempt >= 0)
);

CREATE UNIQUE INDEX workspace_scm_deliveries_external_key
    ON workspace_scm_deliveries (connection_id, external_delivery_id);

CREATE INDEX workspace_scm_deliveries_pending_idx
    ON workspace_scm_deliveries (received_at, id) WHERE processed_at IS NULL;

CREATE INDEX workspace_scm_deliveries_sweep_idx ON workspace_scm_deliveries (received_at);

-- +goose Down
DROP TABLE IF EXISTS workspace_scm_deliveries;

DROP TABLE IF EXISTS workspace_comment_mirrors;

DROP TABLE IF EXISTS workspace_issue_mirrors;

DROP TABLE IF EXISTS workspace_code_links;

DROP TABLE IF EXISTS workspace_team_scm_settings;

DROP TABLE IF EXISTS workspace_scm_connections;
