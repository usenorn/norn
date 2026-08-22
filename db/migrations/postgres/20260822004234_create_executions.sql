-- +goose Up
CREATE TABLE workspace_executions (
    id               text PRIMARY KEY,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id         uuid NOT NULL,
    delegation_id    uuid NOT NULL REFERENCES workspace_issue_delegations (id) ON DELETE CASCADE,
    agent_id         uuid NOT NULL REFERENCES workspace_agents (id) ON DELETE CASCADE,
    runner_id        uuid REFERENCES workspace_runners (id) ON DELETE SET NULL,
    codebase_id      uuid REFERENCES workspace_codebases (id) ON DELETE SET NULL,
    attempt          integer NOT NULL,
    state            text NOT NULL DEFAULT 'queued',
    reason           text NOT NULL DEFAULT '',
    params           jsonb NOT NULL DEFAULT '{}',
    lease_expires_at timestamptz,
    queued_at        timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_executions_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_executions_id_check CHECK (id LIKE 'exec-%'),
    CONSTRAINT workspace_executions_attempt_check CHECK (attempt >= 1),
    CONSTRAINT workspace_executions_params_check CHECK (jsonb_typeof(params) = 'object'),
    CONSTRAINT workspace_executions_state_check
        CHECK (state IN ('queued', 'leased', 'preparing', 'running', 'waiting_for_input',
                         'queued_for_resume', 'finalizing', 'awaiting_review', 'approved',
                         'completed', 'failed', 'cancelled', 'interrupted')),
    CONSTRAINT workspace_executions_finished_check
        CHECK ((finished_at IS NOT NULL)
               = (state IN ('completed', 'failed', 'cancelled', 'interrupted')))
);

CREATE UNIQUE INDEX workspace_executions_attempt_key
    ON workspace_executions (issue_id, attempt);

CREATE INDEX workspace_executions_issue_idx
    ON workspace_executions (issue_id, queued_at DESC, id);

CREATE INDEX workspace_executions_lease_idx
    ON workspace_executions (lease_expires_at) WHERE lease_expires_at IS NOT NULL;

CREATE INDEX workspace_executions_runner_idx
    ON workspace_executions (runner_id)
    WHERE state NOT IN ('completed', 'failed', 'cancelled', 'interrupted');

CREATE TABLE workspace_execution_events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id      text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    sequence          bigint GENERATED ALWAYS AS IDENTITY,
    kind              text NOT NULL,
    from_state        text NOT NULL DEFAULT '',
    to_state          text NOT NULL DEFAULT '',
    actor_kind        text NOT NULL,
    actor_account_id  uuid REFERENCES accounts (id) ON DELETE SET NULL,
    actor_agent_id    uuid,
    actor_runner_id   uuid,
    reason            text NOT NULL DEFAULT '',
    detail            jsonb NOT NULL DEFAULT '{}',
    source_id         text NOT NULL DEFAULT '',
    occurred_at       timestamptz NOT NULL DEFAULT now(),
    recorded_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_events_detail_check CHECK (jsonb_typeof(detail) = 'object'),
    CONSTRAINT workspace_execution_events_kind_check
        CHECK (kind IN ('transition', 'phase', 'command', 'tool', 'service', 'preview', 'note'))
);

CREATE UNIQUE INDEX workspace_execution_events_sequence_key
    ON workspace_execution_events (execution_id, sequence);

CREATE UNIQUE INDEX workspace_execution_events_source_key
    ON workspace_execution_events (execution_id, source_id) WHERE source_id <> '';

-- +goose Down
DROP TABLE workspace_execution_events;

DROP TABLE workspace_executions;
