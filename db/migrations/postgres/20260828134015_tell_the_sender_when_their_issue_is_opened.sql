-- +goose Up
ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_kind_check
    CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting', 'opened'));

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_target_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_target_check
    CHECK (kind NOT IN ('assigned', 'membership', 'opened') OR target_account_id IS NOT NULL);

ALTER TABLE workspace_notification_deliveries
    DROP CONSTRAINT workspace_notification_deliveries_reason_check;

ALTER TABLE workspace_notification_deliveries
    ADD CONSTRAINT workspace_notification_deliveries_reason_check
    CHECK (reason IN ('mentioned', 'approval', 'assigned', 'membership', 'following', 'opened'));

ALTER TABLE workspace_notification_settings
    ADD COLUMN inbox_opened boolean NOT NULL DEFAULT true,
    ADD COLUMN email_opened boolean NOT NULL DEFAULT false;

ALTER TABLE workspace_team_notification_settings
    ADD COLUMN inbox_opened boolean NOT NULL DEFAULT true,
    ADD COLUMN email_opened boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_team_notification_settings
    DROP COLUMN email_opened,
    DROP COLUMN inbox_opened;

ALTER TABLE workspace_notification_settings
    DROP COLUMN email_opened,
    DROP COLUMN inbox_opened;

DELETE FROM workspace_notification_deliveries WHERE reason = 'opened';

DELETE FROM workspace_notification_events WHERE kind = 'opened';

ALTER TABLE workspace_notification_deliveries
    DROP CONSTRAINT workspace_notification_deliveries_reason_check;

ALTER TABLE workspace_notification_deliveries
    ADD CONSTRAINT workspace_notification_deliveries_reason_check
    CHECK (reason IN ('mentioned', 'approval', 'assigned', 'membership', 'following'));

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_target_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_target_check
    CHECK (kind NOT IN ('assigned', 'membership') OR target_account_id IS NOT NULL);

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_kind_check
    CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting'));
