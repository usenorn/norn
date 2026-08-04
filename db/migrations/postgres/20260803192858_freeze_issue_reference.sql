-- +goose Up
CREATE TABLE workspace_issue_numbers (
    team_id     uuid PRIMARY KEY REFERENCES workspace_teams (id) ON DELETE CASCADE,
    next_number integer NOT NULL DEFAULT 1,
    CONSTRAINT workspace_issue_numbers_next_check CHECK (next_number > 0)
);

INSERT INTO workspace_issue_numbers (team_id, next_number)
SELECT t.id, coalesce(max(i.number), 0) + 1
FROM workspace_teams t
LEFT JOIN workspace_issues i ON i.team_id = t.id
GROUP BY t.id;

ALTER TABLE workspace_issues ADD COLUMN reference_key text;

UPDATE workspace_issues i
SET reference_key = t.key
FROM workspace_teams t
WHERE t.id = i.team_id;

ALTER TABLE workspace_issues
    ALTER COLUMN reference_key SET NOT NULL,
    ADD CONSTRAINT workspace_issues_reference_upper_check
        CHECK (reference_key = upper(reference_key)),
    ADD CONSTRAINT workspace_issues_reference_team_fkey
        FOREIGN KEY (workspace_id, reference_key)
        REFERENCES workspace_teams (workspace_id, key);

DROP INDEX workspace_issues_team_number_key;

CREATE UNIQUE INDEX workspace_issues_reference_key
    ON workspace_issues (workspace_id, reference_key, number);

-- +goose Down
DROP INDEX workspace_issues_reference_key;

CREATE UNIQUE INDEX workspace_issues_team_number_key
    ON workspace_issues (team_id, number);

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_reference_team_fkey,
    DROP CONSTRAINT workspace_issues_reference_upper_check,
    DROP COLUMN reference_key;

DROP TABLE workspace_issue_numbers;
