-- +goose Up
ALTER TABLE workspace_issue_activity RENAME TO workspace_activity;

ALTER INDEX workspace_issue_activity_pkey RENAME TO workspace_activity_pkey;
ALTER INDEX workspace_issue_activity_bulk_action_idx RENAME TO workspace_activity_bulk_action_idx;

DROP INDEX workspace_issue_activity_page_idx;
DROP INDEX workspace_issue_activity_version_idx;

ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_kind_check TO workspace_activity_kind_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_state_change_check TO workspace_activity_state_change_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_property_check TO workspace_activity_property_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_triage_check TO workspace_activity_triage_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_actor_account_id_fkey TO workspace_activity_actor_fkey;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_issue_activity_bulk_action_id_fkey TO workspace_activity_bulk_action_fkey;

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_issue_activity_issue_id_fkey,
    DROP COLUMN from_state_id,
    DROP COLUMN to_state_id,
    ALTER COLUMN issue_id DROP NOT NULL,
    ADD COLUMN project_id   uuid,
    ADD COLUMN operation_id uuid,
    ADD COLUMN actor_kind   text NOT NULL DEFAULT 'user';

UPDATE workspace_activity SET operation_id = id;

ALTER TABLE workspace_activity
    ALTER COLUMN operation_id SET NOT NULL,
    ADD CONSTRAINT workspace_activity_subject_check
        CHECK (num_nonnulls(issue_id, project_id) = 1),
    ADD CONSTRAINT workspace_activity_actor_kind_check
        CHECK (actor_kind IN ('user', 'token', 'agent', 'system')),
    ADD CONSTRAINT workspace_activity_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    ADD CONSTRAINT workspace_activity_project_fkey
        FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE CASCADE;

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed'));

CREATE INDEX workspace_activity_issue_page_idx
    ON workspace_activity (issue_id, created_at, id)
    WHERE issue_id IS NOT NULL AND id = operation_id;

CREATE INDEX workspace_activity_project_page_idx
    ON workspace_activity (project_id, created_at, id)
    WHERE project_id IS NOT NULL AND id = operation_id;

CREATE INDEX workspace_activity_operation_idx
    ON workspace_activity (operation_id);

-- +goose Down
DELETE FROM workspace_activity WHERE issue_id IS NULL;

DROP INDEX workspace_activity_operation_idx;
DROP INDEX workspace_activity_project_page_idx;
DROP INDEX workspace_activity_issue_page_idx;

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted'));

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_project_fkey,
    DROP CONSTRAINT workspace_activity_issue_fkey,
    DROP CONSTRAINT workspace_activity_actor_kind_check,
    DROP CONSTRAINT workspace_activity_subject_check,
    DROP COLUMN actor_kind,
    DROP COLUMN operation_id,
    DROP COLUMN project_id,
    ALTER COLUMN issue_id SET NOT NULL,
    ADD COLUMN from_state_id uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    ADD COLUMN to_state_id   uuid REFERENCES workspace_workflow_states (id) ON DELETE SET NULL,
    ADD CONSTRAINT workspace_issue_activity_issue_id_fkey
        FOREIGN KEY (issue_id) REFERENCES workspace_issues (id) ON DELETE CASCADE;

ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_bulk_action_fkey TO workspace_issue_activity_bulk_action_id_fkey;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_actor_fkey TO workspace_issue_activity_actor_account_id_fkey;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_triage_check TO workspace_issue_activity_triage_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_property_check TO workspace_issue_activity_property_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_state_change_check TO workspace_issue_activity_state_change_check;
ALTER TABLE workspace_activity
    RENAME CONSTRAINT workspace_activity_kind_check TO workspace_issue_activity_kind_check;

CREATE INDEX workspace_issue_activity_version_idx
    ON workspace_activity (issue_id, version);

CREATE INDEX workspace_issue_activity_page_idx
    ON workspace_activity (issue_id, created_at DESC, id DESC);

ALTER INDEX workspace_activity_bulk_action_idx RENAME TO workspace_issue_activity_bulk_action_idx;
ALTER INDEX workspace_activity_pkey RENAME TO workspace_issue_activity_pkey;

ALTER TABLE workspace_activity RENAME TO workspace_issue_activity;
