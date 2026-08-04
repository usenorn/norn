-- +goose Up
CREATE TABLE workspace_workflow_states (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  uuid NOT NULL,
    team_id       uuid NOT NULL,
    name          text NOT NULL,
    category      text NOT NULL,
    position      integer NOT NULL,
    is_default    boolean NOT NULL DEFAULT false,
    is_completion boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_workflow_states_category_check
        CHECK (category IN ('not_started', 'active', 'complete', 'abandoned')),
    CONSTRAINT workspace_workflow_states_position_check
        CHECK (position > 0),
    CONSTRAINT workspace_workflow_states_completion_category_check
        CHECK (NOT is_completion OR category = 'complete'),
    CONSTRAINT workspace_workflow_states_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_workflow_states_id_team_key
    ON workspace_workflow_states (id, team_id);

CREATE UNIQUE INDEX workspace_workflow_states_team_name_key
    ON workspace_workflow_states (team_id, lower(name));

CREATE UNIQUE INDEX workspace_workflow_states_team_default_key
    ON workspace_workflow_states (team_id) WHERE is_default;

CREATE UNIQUE INDEX workspace_workflow_states_team_completion_key
    ON workspace_workflow_states (team_id) WHERE is_completion;

CREATE INDEX workspace_workflow_states_team_position_idx
    ON workspace_workflow_states (team_id, position);

INSERT INTO workspace_workflow_states (
    workspace_id, team_id, name, category, position, is_default, is_completion
)
SELECT t.workspace_id, t.id, seed.name, seed.category, seed.position, seed.is_default, seed.is_completion
FROM workspace_teams t
CROSS JOIN (
    VALUES
        ('Backlog',     'not_started', 1, false, false),
        ('Todo',        'not_started', 2, true,  false),
        ('In progress', 'active',      3, false, false),
        ('In review',   'active',      4, false, false),
        ('Done',        'complete',    5, false, true),
        ('Canceled',    'abandoned',   6, false, false)
) AS seed (name, category, position, is_default, is_completion);

-- +goose Down
DROP TABLE workspace_workflow_states;
