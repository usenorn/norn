-- +goose Up
CREATE TABLE workspace_issue_drafts (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id          uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    team_id             uuid REFERENCES workspace_teams (id) ON DELETE SET NULL,
    title               text NOT NULL DEFAULT '',
    description         text NOT NULL DEFAULT '',
    description_doc     jsonb,
    state_id            uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    project_id          uuid REFERENCES workspace_projects (id) ON DELETE SET NULL,
    cycle_id            uuid REFERENCES workspace_cycles (id) ON DELETE SET NULL,
    assignee_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    parent_issue_id     uuid,
    label_ids           uuid[] NOT NULL DEFAULT '{}',
    attachment_ids      uuid[] NOT NULL DEFAULT '{}',
    priority            text NOT NULL DEFAULT 'none',
    estimate            integer,
    due_on              date,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_drafts_parent_fkey
        FOREIGN KEY (parent_issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE SET NULL,
    CONSTRAINT workspace_issue_drafts_title_check
        CHECK (char_length(title) <= 256)
);

CREATE INDEX workspace_issue_drafts_owner_idx
    ON workspace_issue_drafts (workspace_id, account_id, updated_at DESC, id DESC);

-- +goose Down
DROP TABLE workspace_issue_drafts;
