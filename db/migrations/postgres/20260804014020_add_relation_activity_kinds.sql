-- +goose Up
ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed'));

-- +goose Down
ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed'));
