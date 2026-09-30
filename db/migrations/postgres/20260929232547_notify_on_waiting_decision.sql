-- +goose Up
ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting',
                        'decision_waiting'));

-- +goose Down
DELETE FROM workspace_notification_events WHERE kind = 'decision_waiting';

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting'));
