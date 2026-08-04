-- +goose Up
CREATE TABLE workspace_team_triage_settings (
    team_id            uuid PRIMARY KEY,
    workspace_id       uuid NOT NULL,
    route_agents       boolean NOT NULL DEFAULT true,
    route_integrations boolean NOT NULL DEFAULT true,
    route_non_members  boolean NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_triage_settings_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_team_triage_settings_workspace_idx
    ON workspace_team_triage_settings (workspace_id);

ALTER TABLE workspace_issues
    ADD COLUMN triage_state text,
    ADD COLUMN triage_source text,
    ADD COLUMN triage_decided_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    ADD COLUMN triage_decided_at timestamptz;

ALTER TABLE workspace_issues
    ADD CONSTRAINT workspace_issues_triage_state_check
        CHECK (triage_state IS NULL
               OR triage_state IN ('waiting', 'accepted', 'declined', 'merged')),
    ADD CONSTRAINT workspace_issues_triage_source_check
        CHECK (triage_source IS NULL
               OR triage_source IN ('user', 'token', 'agent')),
    ADD CONSTRAINT workspace_issues_triage_decision_check
        CHECK (triage_state IS NULL
               OR triage_state = 'waiting'
               OR triage_decided_at IS NOT NULL);

CREATE INDEX workspace_issues_triage_idx
    ON workspace_issues (workspace_id, team_id, created_at DESC, id DESC)
    WHERE triage_state = 'waiting';

ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged')),
    ADD CONSTRAINT workspace_issue_activity_triage_check
        CHECK (kind <> 'triaged' OR field IS NOT NULL);

-- +goose Down
ALTER TABLE workspace_issue_activity
    DROP CONSTRAINT workspace_issue_activity_triage_check,
    DROP CONSTRAINT workspace_issue_activity_kind_check,
    ADD CONSTRAINT workspace_issue_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed'));

DROP INDEX workspace_issues_triage_idx;

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_triage_decision_check,
    DROP CONSTRAINT workspace_issues_triage_source_check,
    DROP CONSTRAINT workspace_issues_triage_state_check,
    DROP COLUMN triage_decided_at,
    DROP COLUMN triage_decided_by_account_id,
    DROP COLUMN triage_source,
    DROP COLUMN triage_state;

DROP TABLE workspace_team_triage_settings;
