-- +goose Up
CREATE TABLE workspace_issues (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL,
    team_id               uuid NOT NULL,
    number                integer NOT NULL,
    title                 text NOT NULL,
    created_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issues_number_check CHECK (number > 0),
    CONSTRAINT workspace_issues_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_issues_team_number_key
    ON workspace_issues (team_id, number);

CREATE INDEX workspace_issues_page_idx
    ON workspace_issues (workspace_id, created_at DESC, id DESC);

CREATE INDEX workspace_issues_team_idx
    ON workspace_issues (team_id);

-- +goose Down
DROP TABLE workspace_issues;
