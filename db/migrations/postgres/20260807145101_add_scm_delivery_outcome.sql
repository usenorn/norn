-- +goose Up
ALTER TABLE workspace_scm_deliveries
    ADD COLUMN outcome text NOT NULL DEFAULT '',
    ADD COLUMN detail text NOT NULL DEFAULT '';

UPDATE workspace_scm_deliveries
SET outcome = 'processed'
WHERE processed_at IS NOT NULL;

ALTER TABLE workspace_scm_deliveries
    ADD CONSTRAINT workspace_scm_deliveries_outcome_check
        CHECK (outcome IN ('', 'processed', 'applied', 'ignored', 'failed')),
    ADD CONSTRAINT workspace_scm_deliveries_settled_check
        CHECK (processed_at IS NULL OR outcome <> '');

CREATE INDEX workspace_scm_deliveries_log_idx
    ON workspace_scm_deliveries (connection_id, received_at DESC, id);

-- +goose Down
DROP INDEX IF EXISTS workspace_scm_deliveries_log_idx;

ALTER TABLE workspace_scm_deliveries
    DROP CONSTRAINT IF EXISTS workspace_scm_deliveries_settled_check,
    DROP CONSTRAINT IF EXISTS workspace_scm_deliveries_outcome_check,
    DROP COLUMN IF EXISTS detail,
    DROP COLUMN IF EXISTS outcome;
