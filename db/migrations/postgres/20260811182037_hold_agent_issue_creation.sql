-- +goose Up
UPDATE workspace_agent_proposals
SET change = jsonb_strip_nulls(jsonb_build_object(
        'expectedVersion', nullif(change -> 'ExpectedVersion', '0'::jsonb),
        'body', nullif(change ->> 'Body', ''),
        'stateId', change -> 'StateID',
        'title', change -> 'Title',
        'description', change -> 'Description',
        'priority', change -> 'Priority',
        'assigneeId', change -> 'AssigneeID',
        'estimate', change -> 'Estimate',
        'dueOn', change -> 'DueOn',
        'cycleId', change -> 'CycleID',
        'projectId', change -> 'ProjectID',
        'clear', change -> 'Clear',
        'checkIds', change -> 'CheckIDs'
    )),
    reasoning = jsonb_strip_nulls(jsonb_build_object(
        'observed', nullif(reasoning ->> 'Observed', ''),
        'uncertain', nullif(reasoning ->> 'Uncertain', ''),
        'consulted', CASE
            WHEN jsonb_typeof(reasoning -> 'Consulted') = 'array' THEN (
                SELECT jsonb_agg(jsonb_strip_nulls(jsonb_build_object(
                    'label', source ->> 'Label',
                    'url', nullif(source ->> 'URL', '')
                )))
                FROM jsonb_array_elements(reasoning -> 'Consulted') AS source
            )
        END
    ));

ALTER TABLE workspace_team_agent_settings
    ADD COLUMN hold_issue_creation text NOT NULL DEFAULT 'never',
    ADD CONSTRAINT workspace_team_agent_settings_issue_creation_check
        CHECK (hold_issue_creation IN ('never', 'always'));

ALTER TABLE workspace_agent_proposals
    ALTER COLUMN issue_id DROP NOT NULL,
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'issue_create', 'check_set')),
    ADD CONSTRAINT workspace_agent_proposals_issue_check
        CHECK ((action = 'issue_create') = (issue_id IS NULL));

-- +goose Down
DELETE FROM workspace_agent_proposals WHERE action = 'issue_create';

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_issue_check,
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'check_set')),
    ALTER COLUMN issue_id SET NOT NULL;

ALTER TABLE workspace_team_agent_settings
    DROP CONSTRAINT workspace_team_agent_settings_issue_creation_check,
    DROP COLUMN hold_issue_creation;

UPDATE workspace_agent_proposals
SET change = jsonb_strip_nulls(jsonb_build_object(
        'ExpectedVersion', coalesce(change -> 'expectedVersion', '0'::jsonb),
        'Body', coalesce(change ->> 'body', ''),
        'StateID', change -> 'stateId',
        'Title', change -> 'title',
        'Description', change -> 'description',
        'Priority', change -> 'priority',
        'AssigneeID', change -> 'assigneeId',
        'Estimate', change -> 'estimate',
        'DueOn', change -> 'dueOn',
        'CycleID', change -> 'cycleId',
        'ProjectID', change -> 'projectId',
        'Clear', change -> 'clear',
        'CheckIDs', change -> 'checkIds'
    )),
    reasoning = jsonb_strip_nulls(jsonb_build_object(
        'Observed', coalesce(reasoning ->> 'observed', ''),
        'Uncertain', coalesce(reasoning ->> 'uncertain', ''),
        'Consulted', CASE
            WHEN jsonb_typeof(reasoning -> 'consulted') = 'array' THEN (
                SELECT jsonb_agg(jsonb_build_object(
                    'Label', source ->> 'label',
                    'URL', coalesce(source ->> 'url', '')
                ))
                FROM jsonb_array_elements(reasoning -> 'consulted') AS source
            )
        END
    ));
