-- +goose Up
ALTER TABLE workspace_team_agent_settings
    ALTER COLUMN hold_comments DROP DEFAULT,
    ALTER COLUMN hold_state_changes DROP DEFAULT,
    ALTER COLUMN hold_issue_edits DROP DEFAULT;

ALTER TABLE workspace_team_agent_settings
    ALTER COLUMN hold_comments TYPE text
        USING (CASE WHEN hold_comments THEN 'always' ELSE 'never' END),
    ALTER COLUMN hold_state_changes TYPE text
        USING (CASE WHEN hold_state_changes THEN 'always' ELSE 'never' END),
    ALTER COLUMN hold_issue_edits TYPE text
        USING (CASE WHEN hold_issue_edits THEN 'always' ELSE 'never' END);

ALTER TABLE workspace_team_agent_settings
    ALTER COLUMN hold_comments SET DEFAULT 'never',
    ALTER COLUMN hold_state_changes SET DEFAULT 'never',
    ALTER COLUMN hold_issue_edits SET DEFAULT 'never',
    ADD CONSTRAINT workspace_team_agent_settings_comments_check
        CHECK (hold_comments IN ('never', 'always')),
    ADD CONSTRAINT workspace_team_agent_settings_state_changes_check
        CHECK (hold_state_changes IN ('never', 'unless_proven', 'always')),
    ADD CONSTRAINT workspace_team_agent_settings_issue_edits_check
        CHECK (hold_issue_edits IN ('never', 'always'));

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'check_set'));

-- +goose Down
DELETE FROM workspace_agent_proposals WHERE action = 'check_set';

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit'));

ALTER TABLE workspace_team_agent_settings
    DROP CONSTRAINT workspace_team_agent_settings_issue_edits_check,
    DROP CONSTRAINT workspace_team_agent_settings_state_changes_check,
    DROP CONSTRAINT workspace_team_agent_settings_comments_check,
    ALTER COLUMN hold_comments DROP DEFAULT,
    ALTER COLUMN hold_state_changes DROP DEFAULT,
    ALTER COLUMN hold_issue_edits DROP DEFAULT;

ALTER TABLE workspace_team_agent_settings
    ALTER COLUMN hold_comments TYPE boolean USING (hold_comments <> 'never'),
    ALTER COLUMN hold_state_changes TYPE boolean USING (hold_state_changes <> 'never'),
    ALTER COLUMN hold_issue_edits TYPE boolean USING (hold_issue_edits <> 'never');

ALTER TABLE workspace_team_agent_settings
    ALTER COLUMN hold_comments SET DEFAULT false,
    ALTER COLUMN hold_state_changes SET DEFAULT false,
    ALTER COLUMN hold_issue_edits SET DEFAULT false;
