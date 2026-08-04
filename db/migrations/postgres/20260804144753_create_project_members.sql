-- +goose Up
CREATE TABLE workspace_project_members (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    project_id   uuid NOT NULL,
    account_id   uuid NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_project_members_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_project_members_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_project_members_project_account_key
    ON workspace_project_members (project_id, account_id);

CREATE INDEX workspace_project_members_workspace_account_idx
    ON workspace_project_members (workspace_id, account_id);

-- +goose Down
DROP TABLE workspace_project_members;
