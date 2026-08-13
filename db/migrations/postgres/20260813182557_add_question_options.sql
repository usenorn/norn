-- +goose Up
ALTER TABLE workspace_issue_questions
    ADD COLUMN options text[] NOT NULL DEFAULT '{}';

ALTER TABLE workspace_issue_questions
    ADD CONSTRAINT workspace_issue_questions_options_check
        CHECK (
            (cardinality(options) = 0 OR cardinality(options) BETWEEN 2 AND 4)
            AND length(array_to_string(options, '')) <= 320
        );

-- +goose Down
ALTER TABLE workspace_issue_questions
    DROP CONSTRAINT workspace_issue_questions_options_check;

ALTER TABLE workspace_issue_questions
    DROP COLUMN options;
