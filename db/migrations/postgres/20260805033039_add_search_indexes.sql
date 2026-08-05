-- +goose NO TRANSACTION

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_issues_search_idx
    ON workspace_issues USING gin (workspace_id, team_id, search_document);

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_issues_title_trgm_idx
    ON workspace_issues USING gin (title gin_trgm_ops);

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_issue_comments_search_idx
    ON workspace_issue_comments USING gin (workspace_id, search_document);

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_projects_search_idx
    ON workspace_projects USING gin (workspace_id, search_document);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS workspace_projects_search_idx;

DROP INDEX CONCURRENTLY IF EXISTS workspace_issue_comments_search_idx;

DROP INDEX CONCURRENTLY IF EXISTS workspace_issues_title_trgm_idx;

DROP INDEX CONCURRENTLY IF EXISTS workspace_issues_search_idx;
