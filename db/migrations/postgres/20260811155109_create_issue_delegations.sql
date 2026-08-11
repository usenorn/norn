-- +goose Up
CREATE TABLE workspace_issue_delegations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL,
    agent_id uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    brief text NOT NULL DEFAULT '',
    delegated_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    delegated_at timestamptz NOT NULL DEFAULT now(),
    recalled_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    recalled_at timestamptz,
    CONSTRAINT workspace_issue_delegations_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_delegations_recalled_check
        CHECK ((recalled_at IS NULL) = (recalled_by_account_id IS NULL))
);

CREATE UNIQUE INDEX workspace_issue_delegations_open_key
    ON workspace_issue_delegations (issue_id) WHERE recalled_at IS NULL;

CREATE INDEX workspace_issue_delegations_issue_idx
    ON workspace_issue_delegations (issue_id, delegated_at DESC, id);

CREATE INDEX workspace_issue_delegations_agent_idx
    ON workspace_issue_delegations (agent_id) WHERE recalled_at IS NULL;

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed',
                        'code_linked', 'code_unlinked',
                        'delegated', 'recalled'));

-- +goose Down
DELETE FROM workspace_activity WHERE kind IN ('delegated', 'recalled');

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed',
                        'code_linked', 'code_unlinked'));

DROP TABLE workspace_issue_delegations;
