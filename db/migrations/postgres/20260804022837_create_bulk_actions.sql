-- +goose Up
CREATE TABLE workspace_bulk_actions (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    requested_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    change                  jsonb NOT NULL,
    status                  text NOT NULL DEFAULT 'queued',
    expected                integer,
    processed               integer NOT NULL DEFAULT 0,
    started_at              timestamptz,
    finished_at             timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_bulk_actions_status_check
        CHECK (status IN ('queued', 'running', 'complete', 'failed')),
    CONSTRAINT workspace_bulk_actions_progress_check
        CHECK (processed >= 0 AND (expected IS NULL OR processed <= expected))
);

CREATE INDEX workspace_bulk_actions_workspace_idx
    ON workspace_bulk_actions (workspace_id, created_at DESC);

CREATE TABLE workspace_bulk_action_outcomes (
    bulk_action_id uuid NOT NULL REFERENCES workspace_bulk_actions (id) ON DELETE CASCADE,
    issue_id       uuid NOT NULL,
    reference      text NOT NULL,
    outcome        text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bulk_action_id, issue_id),
    CONSTRAINT workspace_bulk_action_outcomes_check
        CHECK (outcome IN ('applied', 'unchanged', 'forbidden', 'not_found', 'conflict', 'invalid'))
);

-- +goose Down
DROP TABLE workspace_bulk_action_outcomes;
DROP TABLE workspace_bulk_actions;
