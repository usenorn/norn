-- +goose Up
ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership',
                        'check_failed', 'gap_declared'));

ALTER TABLE workspace_notification_settings
    ADD COLUMN inbox_checks boolean NOT NULL DEFAULT true,
    ADD COLUMN email_checks boolean NOT NULL DEFAULT false;

ALTER TABLE workspace_team_notification_settings
    ADD COLUMN inbox_checks boolean NOT NULL DEFAULT true,
    ADD COLUMN email_checks boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_team_notification_settings
    DROP COLUMN email_checks,
    DROP COLUMN inbox_checks;

ALTER TABLE workspace_notification_settings
    DROP COLUMN email_checks,
    DROP COLUMN inbox_checks;

DELETE FROM workspace_notification_events WHERE kind IN ('check_failed', 'gap_declared');

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership'));
