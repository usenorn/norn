-- +goose Up
CREATE TABLE workspace_projects (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    slug            text NOT NULL,
    name            text NOT NULL,
    description     text NOT NULL DEFAULT '',
    state           text NOT NULL DEFAULT 'planned',
    lead_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    target_on       date,
    archived_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_projects_state_check
        CHECK (state IN ('planned', 'active', 'paused', 'completed', 'cancelled')),
    CONSTRAINT workspace_projects_archive_check
        CHECK (archived_at IS NULL OR state IN ('completed', 'cancelled')),
    CONSTRAINT workspace_projects_slug_check
        CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE UNIQUE INDEX workspace_projects_id_workspace_key
    ON workspace_projects (id, workspace_id);

CREATE UNIQUE INDEX workspace_projects_slug_key
    ON workspace_projects (workspace_id, slug);

CREATE INDEX workspace_projects_workspace_state_idx
    ON workspace_projects (workspace_id, state);

-- +goose Down
DROP TABLE workspace_projects;
