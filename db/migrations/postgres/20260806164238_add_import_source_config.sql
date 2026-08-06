-- +goose Up
ALTER TABLE workspace_import_runs
    ADD COLUMN source_secret_sealed bytea,
    ADD COLUMN source_settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN unknown_reference_policy text NOT NULL DEFAULT 'skip',
    ADD CONSTRAINT workspace_import_runs_unknown_reference_check
        CHECK (unknown_reference_policy IN ('create', 'skip', 'fail'));

-- +goose Down
ALTER TABLE workspace_import_runs
    DROP CONSTRAINT IF EXISTS workspace_import_runs_unknown_reference_check,
    DROP COLUMN IF EXISTS unknown_reference_policy,
    DROP COLUMN IF EXISTS source_settings,
    DROP COLUMN IF EXISTS source_secret_sealed;
