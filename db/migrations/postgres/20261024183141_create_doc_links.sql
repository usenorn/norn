-- +goose Up
CREATE TABLE workspace_doc_links (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL,
    doc_id                uuid NOT NULL,
    issue_id              uuid,
    project_id            uuid,
    created_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_doc_links_target_check
        CHECK (num_nonnulls(issue_id, project_id) = 1),
    CONSTRAINT workspace_doc_links_doc_fkey
        FOREIGN KEY (doc_id, workspace_id)
        REFERENCES workspace_docs (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_doc_links_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_doc_links_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_doc_links_issue_key
    ON workspace_doc_links (doc_id, issue_id) WHERE issue_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_doc_links_project_key
    ON workspace_doc_links (doc_id, project_id) WHERE project_id IS NOT NULL;

CREATE INDEX workspace_doc_links_issue_idx
    ON workspace_doc_links (issue_id) WHERE issue_id IS NOT NULL;

CREATE INDEX workspace_doc_links_project_idx
    ON workspace_doc_links (project_id) WHERE project_id IS NOT NULL;

-- +goose Down
DROP TABLE workspace_doc_links;
