-- +goose Up
CREATE TABLE workspace_project_status_updates (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        uuid NOT NULL REFERENCES workspace_projects (id) ON DELETE CASCADE,
    author_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    health            text NOT NULL,
    body              text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_project_status_updates_health_check
        CHECK (health IN ('on_track', 'at_risk', 'off_track')),
    CONSTRAINT workspace_project_status_updates_body_check
        CHECK (body <> '')
);

CREATE INDEX workspace_project_status_updates_project_idx
    ON workspace_project_status_updates (project_id, created_at DESC);

-- +goose Down
DROP TABLE workspace_project_status_updates;
