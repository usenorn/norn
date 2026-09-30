-- +goose Up
ALTER TABLE workspace_issue_questions
    ADD COLUMN stage text,
    ADD CONSTRAINT workspace_issue_questions_stage_check
        CHECK (stage IN ('planning', 'implementation'));

UPDATE workspace_issue_questions q
SET stage = CASE WHEN e.stage = 'planning' THEN 'planning' ELSE 'implementation' END
FROM workspace_executions e
WHERE e.id = q.execution_id;

ALTER TABLE workspace_notification_settings
    ADD COLUMN decision_channel text NOT NULL DEFAULT 'norn',
    ADD CONSTRAINT workspace_notification_settings_decision_channel_check
        CHECK (decision_channel IN ('norn', 'telegram'));

ALTER TABLE workspace_telegram_question_messages RENAME TO workspace_telegram_decision_messages;

ALTER TABLE workspace_telegram_decision_messages
    RENAME CONSTRAINT workspace_telegram_question_messages_question_key
        TO workspace_telegram_decision_messages_question_key;

ALTER TABLE workspace_telegram_decision_messages
    ADD COLUMN kind text NOT NULL DEFAULT 'question',
    ADD COLUMN execution_id text REFERENCES workspace_executions (id) ON DELETE CASCADE,
    ADD COLUMN plan_revision integer,
    ADD COLUMN review_heads jsonb,
    ALTER COLUMN question_id DROP NOT NULL,
    ADD CONSTRAINT workspace_telegram_decision_messages_kind_check
        CHECK (kind IN ('question', 'plan', 'review')),
    ADD CONSTRAINT workspace_telegram_decision_messages_subject_check
        CHECK ((kind = 'question' AND question_id IS NOT NULL AND execution_id IS NULL)
            OR (kind = 'plan' AND execution_id IS NOT NULL AND plan_revision IS NOT NULL AND question_id IS NULL)
            OR (kind = 'review' AND execution_id IS NOT NULL AND review_heads IS NOT NULL AND question_id IS NULL));

ALTER TABLE workspace_telegram_decision_messages ALTER COLUMN kind DROP DEFAULT;

CREATE UNIQUE INDEX workspace_telegram_decision_messages_plan_key
    ON workspace_telegram_decision_messages (execution_id, plan_revision, bot_id, chat_id)
    WHERE kind = 'plan';

CREATE UNIQUE INDEX workspace_telegram_decision_messages_review_key
    ON workspace_telegram_decision_messages (execution_id, md5(review_heads::text), bot_id, chat_id)
    WHERE kind = 'review';

-- +goose Down
DELETE FROM workspace_telegram_decision_messages WHERE kind <> 'question';

DROP INDEX workspace_telegram_decision_messages_review_key;
DROP INDEX workspace_telegram_decision_messages_plan_key;

ALTER TABLE workspace_telegram_decision_messages
    DROP CONSTRAINT workspace_telegram_decision_messages_subject_check,
    DROP CONSTRAINT workspace_telegram_decision_messages_kind_check,
    ALTER COLUMN question_id SET NOT NULL,
    DROP COLUMN review_heads,
    DROP COLUMN plan_revision,
    DROP COLUMN execution_id,
    DROP COLUMN kind;

ALTER TABLE workspace_telegram_decision_messages
    RENAME CONSTRAINT workspace_telegram_decision_messages_question_key
        TO workspace_telegram_question_messages_question_key;

ALTER TABLE workspace_telegram_decision_messages RENAME TO workspace_telegram_question_messages;

ALTER TABLE workspace_notification_settings DROP COLUMN decision_channel;

ALTER TABLE workspace_issue_questions DROP COLUMN stage;
