-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN state_id uuid;

UPDATE workspace_issues i
SET state_id = (
    SELECT s.id
    FROM workspace_workflow_states s
    WHERE s.team_id = i.team_id AND s.is_default
    ORDER BY s.position, s.id
    LIMIT 1
);

ALTER TABLE workspace_issues
    ALTER COLUMN state_id SET NOT NULL;

ALTER TABLE workspace_issues
    ADD CONSTRAINT workspace_issues_state_fkey
        FOREIGN KEY (state_id, team_id)
        REFERENCES workspace_workflow_states (id, team_id);

CREATE INDEX workspace_issues_state_idx ON workspace_issues (state_id);

-- +goose Down
ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_state_fkey;

ALTER TABLE workspace_issues
    DROP COLUMN state_id;
