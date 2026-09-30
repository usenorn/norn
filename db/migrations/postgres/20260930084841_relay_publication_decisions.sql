-- +goose Up
ALTER TABLE workspace_telegram_decision_messages
    ADD COLUMN publication_round text,
    DROP CONSTRAINT workspace_telegram_decision_messages_kind_check,
    DROP CONSTRAINT workspace_telegram_decision_messages_subject_check,
    ADD CONSTRAINT workspace_telegram_decision_messages_kind_check
        CHECK (kind IN ('question', 'plan', 'review', 'publication')),
    ADD CONSTRAINT workspace_telegram_decision_messages_subject_check
        CHECK ((kind = 'question' AND question_id IS NOT NULL AND execution_id IS NULL)
            OR (kind = 'plan' AND execution_id IS NOT NULL AND plan_revision IS NOT NULL AND question_id IS NULL)
            OR (kind = 'review' AND execution_id IS NOT NULL AND review_heads IS NOT NULL AND question_id IS NULL)
            OR (kind = 'publication' AND execution_id IS NOT NULL AND publication_round IS NOT NULL
                AND question_id IS NULL));

CREATE UNIQUE INDEX workspace_telegram_decision_messages_publication_key
    ON workspace_telegram_decision_messages (execution_id, publication_round, bot_id, chat_id)
    WHERE kind = 'publication';

-- +goose Down
DELETE FROM workspace_telegram_decision_messages WHERE kind = 'publication';

DROP INDEX workspace_telegram_decision_messages_publication_key;

ALTER TABLE workspace_telegram_decision_messages
    DROP CONSTRAINT workspace_telegram_decision_messages_subject_check,
    DROP CONSTRAINT workspace_telegram_decision_messages_kind_check,
    ADD CONSTRAINT workspace_telegram_decision_messages_kind_check
        CHECK (kind IN ('question', 'plan', 'review')),
    ADD CONSTRAINT workspace_telegram_decision_messages_subject_check
        CHECK ((kind = 'question' AND question_id IS NOT NULL AND execution_id IS NULL)
            OR (kind = 'plan' AND execution_id IS NOT NULL AND plan_revision IS NOT NULL AND question_id IS NULL)
            OR (kind = 'review' AND execution_id IS NOT NULL AND review_heads IS NOT NULL AND question_id IS NULL)),
    DROP COLUMN publication_round;
