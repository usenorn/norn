-- +goose Up
ALTER TABLE workspace_team_notification_settings
    DROP COLUMN IF EXISTS email_opened,
    DROP COLUMN IF EXISTS inbox_opened;

ALTER TABLE workspace_notification_settings
    DROP COLUMN IF EXISTS email_opened,
    DROP COLUMN IF EXISTS inbox_opened;

DELETE FROM workspace_notification_deliveries WHERE reason = 'opened';

DELETE FROM workspace_notification_events WHERE kind = 'opened';

ALTER TABLE workspace_notification_deliveries
    DROP CONSTRAINT IF EXISTS workspace_notification_deliveries_reason_check;

ALTER TABLE workspace_notification_deliveries
    ADD CONSTRAINT workspace_notification_deliveries_reason_check
    CHECK (reason IN ('mentioned', 'approval', 'assigned', 'membership', 'following'));

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT IF EXISTS workspace_notification_events_target_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_target_check
    CHECK (kind NOT IN ('assigned', 'membership') OR target_account_id IS NOT NULL);

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT IF EXISTS workspace_notification_events_kind_check;

ALTER TABLE workspace_notification_events
    ADD CONSTRAINT workspace_notification_events_kind_check
    CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting'));

DROP TABLE IF EXISTS workspace_subject_views;

-- +goose Down
CREATE TABLE workspace_subject_views (
    workspace_id    uuid NOT NULL,
    account_id      uuid NOT NULL,
    subject_kind    text NOT NULL,
    subject_id      uuid NOT NULL,
    first_viewed_at timestamptz NOT NULL DEFAULT now(),
    last_viewed_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id, subject_kind, subject_id),
    CONSTRAINT workspace_subject_views_subject_kind_check
        CHECK (subject_kind IN ('issue', 'project', 'team')),
    CONSTRAINT workspace_subject_views_order_check
        CHECK (last_viewed_at >= first_viewed_at),
    CONSTRAINT workspace_subject_views_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_subject_views_subject_idx
    ON workspace_subject_views (workspace_id, subject_kind, subject_id);

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
