-- +goose Up
ALTER TABLE workspace_code_links
    DROP COLUMN action,
    ADD COLUMN checks text NOT NULL DEFAULT '',
    ADD CONSTRAINT workspace_code_links_checks_check
        CHECK (checks IN ('', 'pending', 'passing', 'failing'));

-- One row per person asked to look at a change. The verdict is kept per reviewer rather
-- than reduced to a single state on the link, because "two approvals and one asking for
-- changes" is exactly the situation somebody opens the issue to understand.
CREATE TABLE workspace_code_link_reviewers (
    link_id uuid NOT NULL REFERENCES workspace_code_links (id) ON DELETE CASCADE,
    login text NOT NULL,
    verdict text NOT NULL,
    url text NOT NULL DEFAULT '',
    reviewed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (link_id, login),
    CONSTRAINT workspace_code_link_reviewers_verdict_check
        CHECK (verdict IN ('requested', 'commented', 'approved', 'changes_requested', 'dismissed'))
);

CREATE INDEX workspace_code_link_reviewers_link_idx
    ON workspace_code_link_reviewers (link_id, login);

CREATE TABLE workspace_team_scm_settings (
    team_id uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    branch_template text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_team_scm_settings_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE INDEX workspace_team_scm_settings_workspace_idx
    ON workspace_team_scm_settings (workspace_id);

-- There is always an issue whose rule is wrong. Suppression is per issue rather than a
-- team-wide switch, so one exception does not cost the team its automation.
ALTER TABLE workspace_issues
    ADD COLUMN scm_automation_suppressed boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_issues
    DROP COLUMN scm_automation_suppressed;

DROP TABLE IF EXISTS workspace_team_scm_settings;

DROP TABLE IF EXISTS workspace_code_link_reviewers;

ALTER TABLE workspace_code_links
    DROP CONSTRAINT workspace_code_links_checks_check,
    DROP COLUMN checks,
    ADD COLUMN action text NOT NULL DEFAULT '';
