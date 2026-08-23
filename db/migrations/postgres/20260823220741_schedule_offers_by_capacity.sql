-- +goose Up
ALTER TABLE workspace_runners
    ADD COLUMN paused_at timestamptz;

ALTER TABLE workspace_executions
    ADD COLUMN queued_reason text NOT NULL DEFAULT '',
    ADD CONSTRAINT workspace_executions_queued_reason_check
        CHECK (queued_reason IN ('', 'no_runner', 'runners_offline', 'runners_paused',
                                 'runners_busy'));

CREATE UNIQUE INDEX workspace_executions_live_delegation_key
    ON workspace_executions (delegation_id)
    WHERE state NOT IN ('completed', 'failed', 'cancelled', 'interrupted');

CREATE INDEX workspace_executions_queued_idx
    ON workspace_executions (agent_id, queued_at, id)
    WHERE state IN ('queued', 'queued_for_resume');

-- +goose Down
DROP INDEX workspace_executions_queued_idx;

DROP INDEX workspace_executions_live_delegation_key;

ALTER TABLE workspace_executions
    DROP CONSTRAINT workspace_executions_queued_reason_check,
    DROP COLUMN queued_reason;

ALTER TABLE workspace_runners
    DROP COLUMN paused_at;
