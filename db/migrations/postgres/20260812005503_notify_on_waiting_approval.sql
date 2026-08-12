-- +goose Up
ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership',
                        'check_failed', 'gap_declared', 'approval_waiting'));

ALTER TABLE workspace_notification_deliveries
    DROP CONSTRAINT workspace_notification_deliveries_reason_check,
    ADD CONSTRAINT workspace_notification_deliveries_reason_check
        CHECK (reason IN ('mentioned', 'approval', 'assigned', 'membership', 'following'));

ALTER TABLE workspace_notification_settings
    ADD COLUMN inbox_approvals boolean NOT NULL DEFAULT true,
    ADD COLUMN email_approvals boolean NOT NULL DEFAULT true;

ALTER TABLE workspace_team_notification_settings
    ADD COLUMN inbox_approvals boolean NOT NULL DEFAULT true,
    ADD COLUMN email_approvals boolean NOT NULL DEFAULT true;

-- +goose Down
ALTER TABLE workspace_team_notification_settings
    DROP COLUMN email_approvals,
    DROP COLUMN inbox_approvals;

ALTER TABLE workspace_notification_settings
    DROP COLUMN email_approvals,
    DROP COLUMN inbox_approvals;

DELETE FROM workspace_notification_deliveries WHERE reason = 'approval';

ALTER TABLE workspace_notification_deliveries
    DROP CONSTRAINT workspace_notification_deliveries_reason_check,
    ADD CONSTRAINT workspace_notification_deliveries_reason_check
        CHECK (reason IN ('mentioned', 'assigned', 'membership', 'following'));

DELETE FROM workspace_notification_events WHERE kind = 'approval_waiting';

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership',
                        'check_failed', 'gap_declared'));
