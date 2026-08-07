-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN rank text COLLATE "C";

UPDATE workspace_issues i
SET rank = ranked.rank
FROM (
    SELECT id, lpad(row_number() OVER (
        PARTITION BY workspace_id ORDER BY created_at DESC, id DESC
    )::text, 12, '0') || 'i' AS rank
    FROM workspace_issues
) ranked
WHERE ranked.id = i.id;

ALTER TABLE workspace_issues
    ALTER COLUMN rank SET NOT NULL;

CREATE INDEX workspace_issues_rank_idx ON workspace_issues (workspace_id, rank);

-- +goose Down
DROP INDEX workspace_issues_rank_idx;

ALTER TABLE workspace_issues
    DROP COLUMN rank;
