-- +goose Up
CREATE TABLE workspace_cycle_scope_changes (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cycle_id         uuid NOT NULL REFERENCES workspace_cycles (id) ON DELETE CASCADE,
    issue_id         uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    change           text NOT NULL,
    actor_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    changed_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_cycle_scope_changes_change_check
        CHECK (change IN ('added', 'removed', 'rolled_over', 'returned'))
);

CREATE INDEX workspace_cycle_scope_changes_cycle_idx
    ON workspace_cycle_scope_changes (cycle_id, changed_at);

-- +goose Down
DROP TABLE workspace_cycle_scope_changes;
