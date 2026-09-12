-- +goose NO TRANSACTION

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_issues_number_idx
    ON workspace_issues (workspace_id, number);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS workspace_issues_number_idx;
