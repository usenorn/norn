-- +goose Up
CREATE TABLE workspace_invitation_teams (
    invitation_id uuid NOT NULL REFERENCES workspace_invitations (id) ON DELETE CASCADE,
    team_id       uuid NOT NULL REFERENCES workspace_teams (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (invitation_id, team_id)
);

CREATE INDEX workspace_invitation_teams_team_idx ON workspace_invitation_teams (team_id);

-- +goose Down
DROP TABLE workspace_invitation_teams;
