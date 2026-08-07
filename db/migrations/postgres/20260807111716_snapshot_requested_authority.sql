-- +goose Up
ALTER TABLE workspace_import_runs
    ADD COLUMN requested_all_teams boolean NOT NULL DEFAULT true,
    ADD COLUMN requested_team_ids text[] NOT NULL DEFAULT '{}',
    ADD COLUMN requested_scopes text[];

ALTER TABLE workspace_bulk_actions
    ADD COLUMN requested_all_teams boolean NOT NULL DEFAULT true,
    ADD COLUMN requested_team_ids text[] NOT NULL DEFAULT '{}',
    ADD COLUMN requested_scopes text[];

-- +goose Down
ALTER TABLE workspace_import_runs
    DROP COLUMN requested_all_teams,
    DROP COLUMN requested_team_ids,
    DROP COLUMN requested_scopes;

ALTER TABLE workspace_bulk_actions
    DROP COLUMN requested_all_teams,
    DROP COLUMN requested_team_ids,
    DROP COLUMN requested_scopes;
