-- +goose Up
ALTER TABLE workspace_memories
    DROP CONSTRAINT workspace_memories_status_check,
    ADD CONSTRAINT workspace_memories_status_check
        CHECK (status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn', 'rejected'));

ALTER TABLE workspace_memory_revisions
    ADD COLUMN reason text NOT NULL DEFAULT '',
    ADD CONSTRAINT workspace_memory_revisions_reason_check CHECK (char_length(reason) <= 1000),
    DROP CONSTRAINT workspace_memory_revisions_status_check,
    ADD CONSTRAINT workspace_memory_revisions_status_check
        CHECK (status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn', 'rejected')),
    DROP CONSTRAINT workspace_memory_revisions_action_check,
    ADD CONSTRAINT workspace_memory_revisions_action_check
        CHECK (action IN ('created', 'corrected', 'reshared', 'accepted', 'disputed', 'superseded', 'withdrawn',
                          'rejected', 'reconciled'));

ALTER TABLE workspace_notification_events
    ADD COLUMN memory_id uuid,
    ADD CONSTRAINT workspace_notification_events_memory_fkey
        FOREIGN KEY (memory_id, workspace_id)
        REFERENCES workspace_memories (id, workspace_id) ON DELETE CASCADE,
    DROP CONSTRAINT workspace_notification_events_subject_check,
    ADD CONSTRAINT workspace_notification_events_subject_check
        CHECK (num_nonnulls(issue_id, project_id, team_id, memory_id) = 1
               AND subject_id = coalesce(issue_id, project_id, team_id, memory_id)
               AND subject_kind = CASE
                                  WHEN issue_id IS NOT NULL THEN 'issue'
                                  WHEN project_id IS NOT NULL THEN 'project'
                                  WHEN team_id IS NOT NULL THEN 'team'
                                  ELSE 'memory'
                                  END),
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting',
                        'decision_waiting', 'memory_proposed')),
    DROP CONSTRAINT workspace_notification_events_target_check,
    ADD CONSTRAINT workspace_notification_events_target_check
        CHECK (kind NOT IN ('assigned', 'membership', 'memory_proposed') OR target_account_id IS NOT NULL);

CREATE INDEX workspace_notification_events_memory_idx ON workspace_notification_events (memory_id)
    WHERE memory_id IS NOT NULL;

ALTER TABLE workspace_notification_reads
    DROP CONSTRAINT workspace_notification_reads_subject_kind_check,
    ADD CONSTRAINT workspace_notification_reads_subject_kind_check
        CHECK (subject_kind IN ('issue', 'project', 'team', 'memory'));

-- +goose Down
DELETE FROM workspace_notification_reads WHERE subject_kind = 'memory';

DELETE FROM workspace_notification_events WHERE memory_id IS NOT NULL OR kind = 'memory_proposed';

ALTER TABLE workspace_notification_reads
    DROP CONSTRAINT workspace_notification_reads_subject_kind_check,
    ADD CONSTRAINT workspace_notification_reads_subject_kind_check
        CHECK (subject_kind IN ('issue', 'project', 'team'));

DROP INDEX IF EXISTS workspace_notification_events_memory_idx;

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_target_check,
    ADD CONSTRAINT workspace_notification_events_target_check
        CHECK (kind NOT IN ('assigned', 'membership') OR target_account_id IS NOT NULL),
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership', 'approval_waiting',
                        'decision_waiting')),
    DROP CONSTRAINT workspace_notification_events_subject_check,
    ADD CONSTRAINT workspace_notification_events_subject_check
        CHECK (num_nonnulls(issue_id, project_id, team_id) = 1
               AND subject_id = coalesce(issue_id, project_id, team_id)
               AND subject_kind = CASE
                                  WHEN issue_id IS NOT NULL THEN 'issue'
                                  WHEN project_id IS NOT NULL THEN 'project'
                                  ELSE 'team'
                                  END),
    DROP CONSTRAINT workspace_notification_events_memory_fkey,
    DROP COLUMN memory_id;

DELETE FROM workspace_memory_revisions WHERE action IN ('rejected', 'reconciled') OR status = 'rejected';

UPDATE workspace_memories SET status = 'withdrawn' WHERE status = 'rejected';

ALTER TABLE workspace_memory_revisions
    DROP CONSTRAINT workspace_memory_revisions_action_check,
    ADD CONSTRAINT workspace_memory_revisions_action_check
        CHECK (action IN ('created', 'corrected', 'reshared', 'accepted', 'disputed', 'superseded', 'withdrawn')),
    DROP CONSTRAINT workspace_memory_revisions_status_check,
    ADD CONSTRAINT workspace_memory_revisions_status_check
        CHECK (status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn')),
    DROP CONSTRAINT workspace_memory_revisions_reason_check,
    DROP COLUMN reason;

ALTER TABLE workspace_memories
    DROP CONSTRAINT workspace_memories_status_check,
    ADD CONSTRAINT workspace_memories_status_check
        CHECK (status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn'));
