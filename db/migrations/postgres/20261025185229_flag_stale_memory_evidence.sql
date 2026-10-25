-- +goose Up
ALTER TABLE workspace_memory_evidence
    ADD COLUMN alerted_state text,
    ADD CONSTRAINT workspace_memory_evidence_alerted_state_check
        CHECK (alerted_state IN ('changed', 'unavailable'));

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting',
                        'decision_waiting', 'memory_proposed', 'memory_evidence_changed')),
    DROP CONSTRAINT workspace_notification_events_target_check,
    ADD CONSTRAINT workspace_notification_events_target_check
        CHECK (kind NOT IN ('assigned', 'membership', 'memory_proposed', 'memory_evidence_changed')
            OR target_account_id IS NOT NULL);

-- +goose Down
DELETE FROM workspace_notification_events WHERE kind = 'memory_evidence_changed';

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_target_check,
    ADD CONSTRAINT workspace_notification_events_target_check
        CHECK (kind NOT IN ('assigned', 'membership', 'memory_proposed') OR target_account_id IS NOT NULL),
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting',
                        'decision_waiting', 'memory_proposed'));

ALTER TABLE workspace_memory_evidence DROP COLUMN alerted_state;
