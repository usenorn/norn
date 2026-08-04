-- +goose Up
CREATE TABLE workspace_team_cycle_cadences (
    team_id      uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    length_weeks integer NOT NULL,
    anchor_on    date NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_cycle_cadences_length_check
        CHECK (length_weeks BETWEEN 1 AND 4),
    CONSTRAINT workspace_team_cycle_cadences_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_team_cycle_cadences_workspace_idx
    ON workspace_team_cycle_cadences (workspace_id);

-- +goose Down
DROP TABLE workspace_team_cycle_cadences;
