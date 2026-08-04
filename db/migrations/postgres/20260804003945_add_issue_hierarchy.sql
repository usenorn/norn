-- +goose Up
ALTER TABLE workspace_issues
    ADD COLUMN parent_issue_id uuid,
    ADD COLUMN depth           integer NOT NULL DEFAULT 1;

ALTER TABLE workspace_issues
    ADD CONSTRAINT workspace_issues_parent_fkey
        FOREIGN KEY (parent_issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id)
        ON DELETE SET NULL (parent_issue_id),
    ADD CONSTRAINT workspace_issues_parent_self_check
        CHECK (parent_issue_id IS NULL OR parent_issue_id <> id),
    ADD CONSTRAINT workspace_issues_depth_check
        CHECK (depth BETWEEN 1 AND 5),
    ADD CONSTRAINT workspace_issues_root_depth_check
        CHECK (parent_issue_id IS NOT NULL OR depth = 1);

CREATE INDEX workspace_issues_parent_idx
    ON workspace_issues (parent_issue_id) WHERE parent_issue_id IS NOT NULL;

-- +goose Down
DROP INDEX workspace_issues_parent_idx;

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_root_depth_check,
    DROP CONSTRAINT workspace_issues_depth_check,
    DROP CONSTRAINT workspace_issues_parent_self_check,
    DROP CONSTRAINT workspace_issues_parent_fkey;

ALTER TABLE workspace_issues
    DROP COLUMN depth,
    DROP COLUMN parent_issue_id;
