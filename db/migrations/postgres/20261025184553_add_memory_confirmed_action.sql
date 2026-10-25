-- +goose Up
ALTER TABLE workspace_memory_revisions
    DROP CONSTRAINT workspace_memory_revisions_action_check,
    ADD CONSTRAINT workspace_memory_revisions_action_check
        CHECK (action IN ('created', 'corrected', 'reshared', 'accepted', 'disputed', 'superseded', 'withdrawn',
                          'rejected', 'reconciled', 'confirmed'));

-- +goose Down
DELETE FROM workspace_memory_revisions WHERE action = 'confirmed';

ALTER TABLE workspace_memory_revisions
    DROP CONSTRAINT workspace_memory_revisions_action_check,
    ADD CONSTRAINT workspace_memory_revisions_action_check
        CHECK (action IN ('created', 'corrected', 'reshared', 'accepted', 'disputed', 'superseded', 'withdrawn',
                          'rejected', 'reconciled'));
