-- +goose Up
ALTER TABLE workspace_executions DROP CONSTRAINT workspace_executions_state_check;

ALTER TABLE workspace_executions
    ADD CONSTRAINT workspace_executions_state_check
        CHECK (state IN ('queued', 'leased', 'preparing', 'running', 'waiting_for_input',
                         'awaiting_plan_approval', 'queued_for_resume', 'finalizing',
                         'awaiting_review', 'approved', 'watching', 'completed', 'failed',
                         'cancelled', 'interrupted'));

-- +goose Down
UPDATE workspace_executions
    SET state = 'completed', finished_at = now(), lease_expires_at = NULL
    WHERE state = 'watching';

ALTER TABLE workspace_executions DROP CONSTRAINT workspace_executions_state_check;

ALTER TABLE workspace_executions
    ADD CONSTRAINT workspace_executions_state_check
        CHECK (state IN ('queued', 'leased', 'preparing', 'running', 'waiting_for_input',
                         'awaiting_plan_approval', 'queued_for_resume', 'finalizing',
                         'awaiting_review', 'approved', 'completed', 'failed', 'cancelled',
                         'interrupted'));
