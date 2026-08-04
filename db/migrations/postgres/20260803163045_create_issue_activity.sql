-- +goose Up
CREATE TABLE workspace_issue_activity (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     uuid NOT NULL,
    issue_id         uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    actor_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    kind             text NOT NULL,
    from_state_id    uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    to_state_id      uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    from_state_name  text NOT NULL DEFAULT '',
    to_state_name    text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed')),
    CONSTRAINT workspace_issue_activity_state_change_check
        CHECK (kind <> 'state_changed' OR (from_state_name <> '' AND to_state_name <> ''))
);

CREATE INDEX workspace_issue_activity_page_idx
    ON workspace_issue_activity (issue_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE workspace_issue_activity;
