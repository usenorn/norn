-- +goose Up
ALTER TABLE workspace_issue_activity
    ADD COLUMN field      text,
    ADD COLUMN from_value text,
    ADD COLUMN to_value   text,
    ADD COLUMN version    integer;

ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored')),
    ADD CONSTRAINT workspace_issue_activity_property_check
        CHECK (kind <> 'property_changed' OR field IS NOT NULL);

CREATE INDEX workspace_issue_activity_version_idx
    ON workspace_issue_activity (issue_id, version);

-- +goose Down
DROP INDEX workspace_issue_activity_version_idx;

ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_property_check,
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed'));

ALTER TABLE workspace_issue_activity
    DROP COLUMN version,
    DROP COLUMN to_value,
    DROP COLUMN from_value,
    DROP COLUMN field;
