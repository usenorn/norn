-- +goose Up
ALTER TABLE workspace_activity
    ADD COLUMN from_state_category text NOT NULL DEFAULT '',
    ADD COLUMN to_state_category text NOT NULL DEFAULT '';

CREATE TABLE workspace_cycle_results (
    cycle_id       uuid NOT NULL REFERENCES workspace_cycles (id) ON DELETE CASCADE,
    issue_id       uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    team_id        uuid NOT NULL REFERENCES workspace_teams (id) ON DELETE CASCADE,
    state_category text NOT NULL,
    state_name     text NOT NULL,
    decision       text NOT NULL DEFAULT '',
    recorded_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cycle_id, issue_id),
    CONSTRAINT workspace_cycle_results_category_check
        CHECK (state_category IN ('not_started', 'active', 'complete', 'abandoned')),
    CONSTRAINT workspace_cycle_results_decision_check
        CHECK (decision IN ('', 'next', 'backlog', 'keep'))
);

CREATE INDEX workspace_cycle_results_issue_idx ON workspace_cycle_results (issue_id);

CREATE TABLE cycle_history_coverage (
    id           boolean PRIMARY KEY DEFAULT true,
    recording_at timestamptz NOT NULL,
    CONSTRAINT cycle_history_coverage_single CHECK (id)
);

INSERT INTO cycle_history_coverage (recording_at) VALUES (now());

ALTER TABLE workspace_cycles
    DROP CONSTRAINT workspace_cycles_rollover_check;

ALTER TABLE workspace_cycles
    ADD CONSTRAINT workspace_cycles_rollover_check
    CHECK (rollover IN ('', 'next', 'backlog', 'keep'));

ALTER TABLE workspace_cycles
    ADD COLUMN owner_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    ADD COLUMN results_recorded_at timestamptz;

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check;

ALTER TABLE workspace_activity
    ADD CONSTRAINT workspace_activity_kind_check
    CHECK (kind IN ('created', 'state_changed', 'state_reclassified', 'property_changed',
                    'team_moved', 'archived', 'unarchived', 'deleted', 'restored',
                    'child_added', 'child_removed', 'relation_added', 'relation_removed',
                    'triaged', 'commented', 'comment_deleted', 'member_added', 'member_removed',
                    'attachment_added', 'attachment_removed', 'code_linked', 'code_unlinked',
                    'delegated', 'recalled', 'question_asked', 'question_answered'));

-- +goose Down
DELETE FROM workspace_activity WHERE kind = 'state_reclassified';

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check;

ALTER TABLE workspace_activity
    ADD CONSTRAINT workspace_activity_kind_check
    CHECK (kind IN ('created', 'state_changed', 'property_changed',
                    'team_moved', 'archived', 'unarchived', 'deleted', 'restored',
                    'child_added', 'child_removed', 'relation_added', 'relation_removed',
                    'triaged', 'commented', 'comment_deleted', 'member_added', 'member_removed',
                    'attachment_added', 'attachment_removed', 'code_linked', 'code_unlinked',
                    'delegated', 'recalled', 'question_asked', 'question_answered'));

UPDATE workspace_cycles SET rollover = '' WHERE rollover = 'keep';

ALTER TABLE workspace_cycles
    DROP CONSTRAINT workspace_cycles_rollover_check;

ALTER TABLE workspace_cycles
    ADD CONSTRAINT workspace_cycles_rollover_check
    CHECK (rollover IN ('', 'next', 'backlog'));

DROP TABLE cycle_history_coverage;

DROP TABLE workspace_cycle_results;

ALTER TABLE workspace_cycles
    DROP COLUMN owner_account_id,
    DROP COLUMN results_recorded_at;

ALTER TABLE workspace_activity
    DROP COLUMN from_state_category,
    DROP COLUMN to_state_category;
