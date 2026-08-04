-- +goose Up
CREATE INDEX workspace_issues_scope_page_idx
    ON workspace_issues (workspace_id, status, created_at DESC, id DESC)
    INCLUDE (team_id);

CREATE INDEX workspace_issues_scope_due_idx
    ON workspace_issues (workspace_id, status, due_on, id DESC)
    INCLUDE (team_id);

-- +goose Down
DROP INDEX workspace_issues_scope_due_idx;

DROP INDEX workspace_issues_scope_page_idx;
