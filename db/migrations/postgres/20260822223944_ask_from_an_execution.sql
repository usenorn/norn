-- +goose Up
ALTER TABLE workspace_issue_questions
    ADD COLUMN execution_id    text REFERENCES workspace_executions (id) ON DELETE SET NULL,
    ADD COLUMN runner_ref      text NOT NULL DEFAULT '',
    ADD COLUMN kind            text NOT NULL DEFAULT 'clarification',
    ADD COLUMN blocking        boolean NOT NULL DEFAULT false,
    ADD COLUMN options         jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN allow_free_text boolean NOT NULL DEFAULT true,
    ADD COLUMN context         jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN state           text NOT NULL DEFAULT 'asked',
    ADD COLUMN settled_at      timestamptz,
    ADD COLUMN settled_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL;

UPDATE workspace_issue_questions SET state = 'answered' WHERE answered_at IS NOT NULL;
UPDATE workspace_issue_questions
SET settled_at = answered_at, settled_by_account_id = answered_by_account_id
WHERE answered_at IS NOT NULL;

ALTER TABLE workspace_issue_questions
    DROP CONSTRAINT workspace_issue_questions_default_check,
    ADD CONSTRAINT workspace_issue_questions_default_check
        CHECK (blocking OR default_answer <> ''),
    ADD CONSTRAINT workspace_issue_questions_kind_check
        CHECK (kind IN ('decision', 'clarification', 'approval')),
    ADD CONSTRAINT workspace_issue_questions_state_check
        CHECK (state IN ('asked', 'answered', 'dismissed', 'expired')),
    ADD CONSTRAINT workspace_issue_questions_answered_state_check
        CHECK ((state = 'answered') = (answered_at IS NOT NULL)),
    ADD CONSTRAINT workspace_issue_questions_settled_check
        CHECK ((settled_at IS NULL) = (state = 'asked')),
    ADD CONSTRAINT workspace_issue_questions_options_check
        CHECK (jsonb_typeof(options) = 'array' AND jsonb_array_length(options) <= 8),
    ADD CONSTRAINT workspace_issue_questions_context_check
        CHECK (jsonb_typeof(context) = 'object'),
    ADD CONSTRAINT workspace_issue_questions_reachable_check
        CHECK (allow_free_text OR jsonb_array_length(options) > 0),
    ADD CONSTRAINT workspace_issue_questions_ref_check
        CHECK ((runner_ref <> '') = (execution_id IS NOT NULL));

CREATE UNIQUE INDEX workspace_issue_questions_ref_key
    ON workspace_issue_questions (execution_id, runner_ref) WHERE runner_ref <> '';

CREATE INDEX workspace_issue_questions_execution_idx
    ON workspace_issue_questions (execution_id, created_at, id) WHERE execution_id IS NOT NULL;

CREATE INDEX workspace_issue_questions_standing_idx
    ON workspace_issue_questions (deadline, id) WHERE state = 'asked';

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed',
                        'code_linked', 'code_unlinked',
                        'delegated', 'recalled',
                        'question_asked', 'question_answered'));

ALTER TABLE workspace_execution_events
    DROP CONSTRAINT workspace_execution_events_kind_check,
    ADD CONSTRAINT workspace_execution_events_kind_check
        CHECK (kind IN ('transition', 'phase', 'command', 'tool', 'service', 'preview',
                        'question', 'note'));

-- +goose Down
ALTER TABLE workspace_execution_events
    DROP CONSTRAINT workspace_execution_events_kind_check,
    ADD CONSTRAINT workspace_execution_events_kind_check
        CHECK (kind IN ('transition', 'phase', 'command', 'tool', 'service', 'preview', 'note'));

DELETE FROM workspace_activity WHERE kind IN ('question_asked', 'question_answered');

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed',
                        'code_linked', 'code_unlinked',
                        'delegated', 'recalled'));

DROP INDEX workspace_issue_questions_standing_idx;
DROP INDEX workspace_issue_questions_execution_idx;
DROP INDEX workspace_issue_questions_ref_key;

DELETE FROM workspace_issue_questions WHERE execution_id IS NOT NULL;

ALTER TABLE workspace_issue_questions
    DROP CONSTRAINT workspace_issue_questions_ref_check,
    DROP CONSTRAINT workspace_issue_questions_reachable_check,
    DROP CONSTRAINT workspace_issue_questions_context_check,
    DROP CONSTRAINT workspace_issue_questions_options_check,
    DROP CONSTRAINT workspace_issue_questions_settled_check,
    DROP CONSTRAINT workspace_issue_questions_answered_state_check,
    DROP CONSTRAINT workspace_issue_questions_state_check,
    DROP CONSTRAINT workspace_issue_questions_kind_check,
    DROP CONSTRAINT workspace_issue_questions_default_check,
    ADD CONSTRAINT workspace_issue_questions_default_check
        CHECK (default_answer <> ''),
    DROP COLUMN settled_by_account_id,
    DROP COLUMN settled_at,
    DROP COLUMN state,
    DROP COLUMN context,
    DROP COLUMN allow_free_text,
    DROP COLUMN options,
    DROP COLUMN blocking,
    DROP COLUMN kind,
    DROP COLUMN runner_ref,
    DROP COLUMN execution_id;
