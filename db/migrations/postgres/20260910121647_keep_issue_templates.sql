-- +goose Up
CREATE TABLE workspace_issue_templates (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    team_id             uuid REFERENCES workspace_teams (id) ON DELETE CASCADE,
    name                text NOT NULL,
    description         text NOT NULL DEFAULT '',
    title               text NOT NULL DEFAULT '',
    body                text NOT NULL DEFAULT '',
    body_doc            jsonb,
    required_fields     text[] NOT NULL DEFAULT '{}',
    state_id            uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    project_id          uuid REFERENCES workspace_projects (id) ON DELETE SET NULL,
    assignee_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    label_ids           uuid[] NOT NULL DEFAULT '{}',
    priority            text NOT NULL DEFAULT 'none',
    estimate            integer,
    position            integer NOT NULL DEFAULT 0,
    created_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_templates_name_check
        CHECK (char_length(name) BETWEEN 1 AND 128)
);

CREATE INDEX workspace_issue_templates_team_idx
    ON workspace_issue_templates (workspace_id, team_id, position, name);

CREATE UNIQUE INDEX workspace_issue_templates_name_key
    ON workspace_issue_templates (workspace_id, coalesce(team_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name));

-- +goose Down
DROP TABLE workspace_issue_templates;
