-- +goose Up
CREATE TABLE workspace_notification_events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    subject_kind      text NOT NULL,
    subject_id        uuid NOT NULL,
    issue_id          uuid,
    project_id        uuid,
    team_id           uuid,
    kind              text NOT NULL,
    actor_account_id  uuid REFERENCES accounts (id) ON DELETE SET NULL,
    actor_kind        text NOT NULL,
    target_account_id uuid REFERENCES accounts (id) ON DELETE CASCADE,
    comment_id        uuid,
    bulk_action_id    uuid REFERENCES workspace_bulk_actions (id) ON DELETE CASCADE,
    fanned_out_at     timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership')),
    CONSTRAINT workspace_notification_events_actor_kind_check
        CHECK (actor_kind IN ('user', 'token', 'agent', 'system')),
    CONSTRAINT workspace_notification_events_subject_check
        CHECK (num_nonnulls(issue_id, project_id, team_id) = 1
               AND subject_id = coalesce(issue_id, project_id, team_id)
               AND subject_kind = CASE
                                  WHEN issue_id IS NOT NULL THEN 'issue'
                                  WHEN project_id IS NOT NULL THEN 'project'
                                  ELSE 'team'
                                  END),
    CONSTRAINT workspace_notification_events_target_check
        CHECK (kind NOT IN ('assigned', 'membership') OR target_account_id IS NOT NULL),
    CONSTRAINT workspace_notification_events_comment_check
        CHECK (kind <> 'commented' OR comment_id IS NOT NULL),
    CONSTRAINT workspace_notification_events_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_notification_events_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_notification_events_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_notification_events_comment_fkey
        FOREIGN KEY (comment_id, workspace_id)
        REFERENCES workspace_issue_comments (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_notification_events_pending_idx
    ON workspace_notification_events (created_at, id)
    WHERE fanned_out_at IS NULL;

CREATE INDEX workspace_notification_events_subject_idx
    ON workspace_notification_events (workspace_id, subject_kind, subject_id, created_at DESC);

CREATE TABLE workspace_notification_deliveries (
    event_id     uuid NOT NULL REFERENCES workspace_notification_events (id) ON DELETE CASCADE,
    account_id   uuid NOT NULL,
    workspace_id uuid NOT NULL,
    reason       text NOT NULL,
    inbox        boolean NOT NULL,
    email        boolean NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, account_id),
    CONSTRAINT workspace_notification_deliveries_reason_check
        CHECK (reason IN ('mentioned', 'assigned', 'membership', 'following')),
    CONSTRAINT workspace_notification_deliveries_channel_check
        CHECK (inbox OR email),
    CONSTRAINT workspace_notification_deliveries_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_notification_deliveries_account_idx
    ON workspace_notification_deliveries (workspace_id, account_id, created_at DESC);

CREATE TABLE workspace_notification_reads (
    workspace_id  uuid NOT NULL,
    account_id    uuid NOT NULL,
    subject_kind  text NOT NULL,
    subject_id    uuid NOT NULL,
    read_through  timestamptz,
    snoozed_until timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id, subject_kind, subject_id),
    CONSTRAINT workspace_notification_reads_subject_kind_check
        CHECK (subject_kind IN ('issue', 'project', 'team')),
    CONSTRAINT workspace_notification_reads_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE TABLE workspace_issue_followers (
    issue_id     uuid NOT NULL,
    workspace_id uuid NOT NULL,
    account_id   uuid NOT NULL,
    state        text NOT NULL DEFAULT 'following',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, account_id),
    CONSTRAINT workspace_issue_followers_state_check
        CHECK (state IN ('following', 'muted')),
    CONSTRAINT workspace_issue_followers_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_followers_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_issue_followers_account_idx
    ON workspace_issue_followers (workspace_id, account_id);

CREATE TABLE workspace_notification_settings (
    workspace_id        uuid NOT NULL,
    account_id          uuid NOT NULL,
    inbox_assigned      boolean NOT NULL DEFAULT true,
    inbox_mentioned     boolean NOT NULL DEFAULT true,
    inbox_commented     boolean NOT NULL DEFAULT true,
    inbox_state_changed boolean NOT NULL DEFAULT true,
    inbox_membership    boolean NOT NULL DEFAULT true,
    inbox_agents        boolean NOT NULL DEFAULT true,
    email_assigned      boolean NOT NULL DEFAULT true,
    email_mentioned     boolean NOT NULL DEFAULT true,
    email_commented     boolean NOT NULL DEFAULT false,
    email_state_changed boolean NOT NULL DEFAULT false,
    email_membership    boolean NOT NULL DEFAULT false,
    email_agents        boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id),
    CONSTRAINT workspace_notification_settings_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE TABLE workspace_team_notification_settings (
    workspace_id        uuid NOT NULL,
    account_id          uuid NOT NULL,
    team_id             uuid NOT NULL,
    inbox_assigned      boolean NOT NULL DEFAULT true,
    inbox_mentioned     boolean NOT NULL DEFAULT true,
    inbox_commented     boolean NOT NULL DEFAULT true,
    inbox_state_changed boolean NOT NULL DEFAULT true,
    inbox_membership    boolean NOT NULL DEFAULT true,
    inbox_agents        boolean NOT NULL DEFAULT true,
    email_assigned      boolean NOT NULL DEFAULT true,
    email_mentioned     boolean NOT NULL DEFAULT true,
    email_commented     boolean NOT NULL DEFAULT false,
    email_state_changed boolean NOT NULL DEFAULT false,
    email_membership    boolean NOT NULL DEFAULT false,
    email_agents        boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id, team_id),
    CONSTRAINT workspace_team_notification_settings_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE,
    CONSTRAINT workspace_team_notification_settings_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE workspace_notification_digests (
    workspace_id      uuid NOT NULL,
    account_id        uuid NOT NULL,
    window_started_at timestamptz NOT NULL,
    sent_at           timestamptz,
    failed_at         timestamptz,
    failure           text NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id, window_started_at),
    CONSTRAINT workspace_notification_digests_outcome_check
        CHECK (num_nonnulls(sent_at, failed_at) <= 1),
    CONSTRAINT workspace_notification_digests_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_notification_digests_failed_idx
    ON workspace_notification_digests (workspace_id, failed_at DESC)
    WHERE failed_at IS NOT NULL;

-- +goose Down
DROP TABLE workspace_notification_digests;

DROP TABLE workspace_team_notification_settings;

DROP TABLE workspace_notification_settings;

DROP TABLE workspace_issue_followers;

DROP TABLE workspace_notification_reads;

DROP TABLE workspace_notification_deliveries;

DROP TABLE workspace_notification_events;
