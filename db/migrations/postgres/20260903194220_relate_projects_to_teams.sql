-- +goose Up
CREATE TABLE workspace_project_teams (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    project_id   uuid NOT NULL,
    team_id      uuid NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_project_teams_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_project_teams_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_project_teams_project_team_key
    ON workspace_project_teams (project_id, team_id);

CREATE INDEX workspace_project_teams_workspace_team_idx
    ON workspace_project_teams (workspace_id, team_id);

-- +goose Down
DROP TABLE workspace_project_teams;
