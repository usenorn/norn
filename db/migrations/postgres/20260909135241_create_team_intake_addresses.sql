-- +goose Up
CREATE TABLE workspace_team_intake_addresses (
    team_id      uuid PRIMARY KEY,
    workspace_id uuid NOT NULL,
    local_part   text NOT NULL,
    domain       text NOT NULL,
    enabled_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    rotated_at   timestamptz,
    CONSTRAINT workspace_team_intake_addresses_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_team_intake_addresses_local_part_idx
    ON workspace_team_intake_addresses (lower(local_part), lower(domain));

CREATE INDEX workspace_team_intake_addresses_workspace_idx
    ON workspace_team_intake_addresses (workspace_id);

CREATE TABLE workspace_intake_deliveries (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    team_id      uuid NOT NULL,
    external_id  text NOT NULL,
    recipient    text NOT NULL,
    sender       text NOT NULL,
    subject      text NOT NULL DEFAULT '',
    received_at  timestamptz NOT NULL,
    processed_at timestamptz,
    outcome      text NOT NULL DEFAULT '',
    issue_id     uuid REFERENCES workspace_issues (id) ON DELETE SET NULL,
    failure      text NOT NULL DEFAULT '',
    CONSTRAINT workspace_intake_deliveries_outcome_check
        CHECK (outcome IN ('', 'filed', 'ignored', 'failed')),
    CONSTRAINT workspace_intake_deliveries_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_intake_deliveries_external_idx
    ON workspace_intake_deliveries (external_id);

CREATE INDEX workspace_intake_deliveries_team_idx
    ON workspace_intake_deliveries (workspace_id, team_id, received_at DESC);

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_triage_source_check,
    ADD CONSTRAINT workspace_issues_triage_source_check
        CHECK (triage_source IS NULL
               OR triage_source IN ('user', 'token', 'agent', 'email'));

-- +goose Down
UPDATE workspace_issues SET triage_source = 'token' WHERE triage_source = 'email';

ALTER TABLE workspace_issues
    DROP CONSTRAINT workspace_issues_triage_source_check,
    ADD CONSTRAINT workspace_issues_triage_source_check
        CHECK (triage_source IS NULL
               OR triage_source IN ('user', 'token', 'agent'));

DROP TABLE workspace_intake_deliveries;

DROP TABLE workspace_team_intake_addresses;
