-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN cycle_id uuid,
    ADD CONSTRAINT workspace_issues_cycle_fkey
        FOREIGN KEY (cycle_id, team_id)
        REFERENCES workspace_cycles (id, team_id) ON DELETE SET NULL (cycle_id);

CREATE INDEX workspace_issues_cycle_idx
    ON workspace_issues (cycle_id) WHERE cycle_id IS NOT NULL;

-- +goose Down
ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_cycle_fkey,
    DROP COLUMN cycle_id;
