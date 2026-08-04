-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE workspace_cycles (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    team_id      uuid NOT NULL,
    number       integer NOT NULL,
    starts_on    date NOT NULL,
    ends_on      date NOT NULL,
    closed_at    timestamptz,
    closed_by    uuid REFERENCES accounts (id) ON DELETE SET NULL,
    rollover     text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_cycles_window_check
        CHECK (ends_on >= starts_on),
    CONSTRAINT workspace_cycles_number_check
        CHECK (number > 0),
    CONSTRAINT workspace_cycles_rollover_check
        CHECK (rollover IN ('', 'next', 'backlog')),
    CONSTRAINT workspace_cycles_closed_check
        CHECK (closed_at IS NOT NULL OR rollover = ''),
    CONSTRAINT workspace_cycles_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_cycles_no_overlap EXCLUDE USING gist (
        team_id WITH =,
        daterange(starts_on, ends_on, '[]') WITH &&
    )
);

CREATE UNIQUE INDEX workspace_cycles_team_number_key
    ON workspace_cycles (team_id, number);

CREATE UNIQUE INDEX workspace_cycles_id_team_key
    ON workspace_cycles (id, team_id);

CREATE INDEX workspace_cycles_team_window_idx
    ON workspace_cycles (team_id, starts_on);

-- +goose Down
DROP TABLE workspace_cycles;
