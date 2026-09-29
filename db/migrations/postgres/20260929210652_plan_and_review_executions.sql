-- +goose Up
ALTER TABLE workspace_executions
    ADD COLUMN stage text NOT NULL DEFAULT 'planning',
    ADD CONSTRAINT workspace_executions_stage_check
        CHECK (stage IN ('planning', 'implementation', 'review', 'publication'));

UPDATE workspace_executions
SET stage = CASE
    WHEN state = 'awaiting_review' THEN 'review'
    WHEN state IN ('approved', 'completed') THEN 'publication'
    ELSE 'implementation'
END;

ALTER TABLE workspace_executions DROP CONSTRAINT workspace_executions_state_check;

ALTER TABLE workspace_executions
    ADD CONSTRAINT workspace_executions_state_check
        CHECK (state IN ('queued', 'leased', 'preparing', 'running', 'waiting_for_input',
                         'awaiting_plan_approval', 'queued_for_resume', 'finalizing',
                         'awaiting_review', 'approved', 'completed', 'failed', 'cancelled',
                         'interrupted'));

CREATE TABLE workspace_execution_plans (
    id                         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id               text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id               uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    revision                   integer NOT NULL,
    ref                        text NOT NULL,
    body                       text NOT NULL,
    proposed_at                timestamptz NOT NULL,
    approved_by_account_id     uuid REFERENCES accounts (id) ON DELETE SET NULL,
    approved_at                timestamptz,
    revision_feedback          text NOT NULL DEFAULT '',
    revision_requested_by      uuid REFERENCES accounts (id) ON DELETE SET NULL,
    revision_requested_at      timestamptz,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_plans_revision_check CHECK (revision >= 1),
    CONSTRAINT workspace_execution_plans_ref_check CHECK (ref <> ''),
    CONSTRAINT workspace_execution_plans_body_check CHECK (body <> ''),
    CONSTRAINT workspace_execution_plans_decided_once_check
        CHECK (approved_at IS NULL OR revision_requested_at IS NULL)
);

CREATE UNIQUE INDEX workspace_execution_plans_revision_key
    ON workspace_execution_plans (execution_id, revision);

CREATE UNIQUE INDEX workspace_execution_plans_ref_key
    ON workspace_execution_plans (execution_id, ref);

CREATE TABLE workspace_execution_reviews (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id      text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    verdict           text NOT NULL,
    summary           text NOT NULL DEFAULT '',
    heads             jsonb NOT NULL DEFAULT '{}',
    author_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    submitted_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_reviews_verdict_check
        CHECK (verdict IN ('comment', 'approve', 'request_changes')),
    CONSTRAINT workspace_execution_reviews_heads_check CHECK (jsonb_typeof(heads) = 'object')
);

CREATE INDEX workspace_execution_reviews_execution_idx
    ON workspace_execution_reviews (execution_id, submitted_at, id);

CREATE TABLE workspace_execution_review_comments (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id      text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    review_id         uuid REFERENCES workspace_execution_reviews (id) ON DELETE CASCADE,
    parent_id         uuid REFERENCES workspace_execution_review_comments (id) ON DELETE CASCADE,
    repository        text NOT NULL,
    path              text NOT NULL,
    side              text NOT NULL,
    line              integer NOT NULL,
    head_sha          text NOT NULL,
    hunk              text NOT NULL DEFAULT '',
    body              text NOT NULL,
    author_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    edited_at         timestamptz,
    resolved_at       timestamptz,
    resolved_by       uuid REFERENCES accounts (id) ON DELETE SET NULL,
    CONSTRAINT workspace_execution_review_comments_side_check CHECK (side IN ('old', 'new')),
    CONSTRAINT workspace_execution_review_comments_line_check CHECK (line >= 1),
    CONSTRAINT workspace_execution_review_comments_anchor_check
        CHECK (repository <> '' AND path <> ''),
    CONSTRAINT workspace_execution_review_comments_body_check CHECK (body <> '')
);

CREATE INDEX workspace_execution_review_comments_execution_idx
    ON workspace_execution_review_comments (execution_id, created_at, id);

CREATE INDEX workspace_execution_review_comments_review_idx
    ON workspace_execution_review_comments (review_id) WHERE review_id IS NOT NULL;

-- +goose Down
DROP TABLE workspace_execution_review_comments;

DROP TABLE workspace_execution_reviews;

DROP TABLE workspace_execution_plans;

DELETE FROM workspace_executions WHERE state = 'awaiting_plan_approval';

ALTER TABLE workspace_executions DROP CONSTRAINT workspace_executions_state_check;

ALTER TABLE workspace_executions
    ADD CONSTRAINT workspace_executions_state_check
        CHECK (state IN ('queued', 'leased', 'preparing', 'running', 'waiting_for_input',
                         'queued_for_resume', 'finalizing', 'awaiting_review', 'approved',
                         'completed', 'failed', 'cancelled', 'interrupted'));

ALTER TABLE workspace_executions DROP COLUMN stage;
