-- +goose Up
CREATE TABLE workspace_issue_questions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL,
    question text NOT NULL,
    default_answer text NOT NULL,
    deadline timestamptz NOT NULL,
    answer text NOT NULL DEFAULT '',
    asked_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    actor_kind text NOT NULL DEFAULT 'agent',
    answered_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    answered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_questions_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_questions_question_check CHECK (question <> ''),
    CONSTRAINT workspace_issue_questions_default_check CHECK (default_answer <> ''),
    CONSTRAINT workspace_issue_questions_actor_kind_check
        CHECK (actor_kind IN ('user', 'token', 'agent', 'system')),
    CONSTRAINT workspace_issue_questions_answered_check
        CHECK ((answered_at IS NULL) = (answer = ''))
);

CREATE INDEX workspace_issue_questions_issue_idx
    ON workspace_issue_questions (issue_id, created_at, id);

-- +goose Down
DROP TABLE workspace_issue_questions;
