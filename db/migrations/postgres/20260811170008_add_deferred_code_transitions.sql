-- +goose Up
ALTER TABLE workspace_code_link_transitions
    ADD COLUMN state_id uuid REFERENCES workspace_workflow_states (id) ON DELETE CASCADE,
    ADD COLUMN status text NOT NULL DEFAULT 'applied',
    ADD COLUMN blocked_by text NOT NULL DEFAULT '',
    ADD COLUMN deferred_at timestamptz,
    ADD CONSTRAINT workspace_code_link_transitions_status_check
        CHECK (status IN ('applied', 'deferred')),
    ADD CONSTRAINT workspace_code_link_transitions_deferred_check
        CHECK ((status = 'deferred') = (deferred_at IS NOT NULL));

CREATE INDEX workspace_code_link_transitions_deferred_idx
    ON workspace_code_link_transitions (issue_id, deferred_at)
    WHERE status = 'deferred';

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
                        'delegated', 'recalled',
                        'check_added', 'check_removed', 'check_approved', 'check_declined',
                        'check_waived', 'check_gap_declared', 'evidence_added',
                        'checks_overridden'));

-- +goose Down
DELETE FROM workspace_activity WHERE kind = 'checks_overridden';

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
                        'delegated', 'recalled',
                        'check_added', 'check_removed', 'check_approved', 'check_declined',
                        'check_waived', 'check_gap_declared', 'evidence_added'));

DROP INDEX workspace_code_link_transitions_deferred_idx;

ALTER TABLE workspace_code_link_transitions
    DROP CONSTRAINT workspace_code_link_transitions_deferred_check,
    DROP CONSTRAINT workspace_code_link_transitions_status_check,
    DROP COLUMN deferred_at,
    DROP COLUMN blocked_by,
    DROP COLUMN status,
    DROP COLUMN state_id;
