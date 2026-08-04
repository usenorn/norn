-- +goose Up
CREATE TABLE workspace_issue_relations (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL,
    source_issue_id       uuid NOT NULL,
    target_issue_id       uuid NOT NULL,
    kind                  text NOT NULL,
    created_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_relations_kind_check
        CHECK (kind IN ('blocks', 'duplicates', 'relates_to')),
    CONSTRAINT workspace_issue_relations_self_check
        CHECK (source_issue_id <> target_issue_id),
    CONSTRAINT workspace_issue_relations_source_fkey
        FOREIGN KEY (source_issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_relations_target_fkey
        FOREIGN KEY (target_issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_issue_relations_pair_key
    ON workspace_issue_relations
       ((least(source_issue_id, target_issue_id)), (greatest(source_issue_id, target_issue_id)));

CREATE INDEX workspace_issue_relations_target_idx
    ON workspace_issue_relations (target_issue_id, kind);

CREATE INDEX workspace_issue_relations_source_idx
    ON workspace_issue_relations (source_issue_id, kind);

-- +goose Down
DROP TABLE workspace_issue_relations;
