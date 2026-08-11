-- +goose Up
ALTER TABLE workspace_issue_checks
    ADD COLUMN expiry_announced_for uuid
        REFERENCES workspace_check_evidence (id) ON DELETE SET NULL;

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
                        'checks_overridden', 'check_expired'));

-- +goose Down
DELETE FROM workspace_activity WHERE kind = 'check_expired';

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

ALTER TABLE workspace_issue_checks DROP COLUMN expiry_announced_for;
