-- +goose Up
ALTER TABLE workspace_codebases
    ADD COLUMN preview_gateway text NOT NULL DEFAULT 'unconfigured';

ALTER TABLE workspace_codebases
    ADD CONSTRAINT workspace_codebases_preview_gateway_check
        CHECK (preview_gateway IN ('reachable', 'unreachable', 'unconfigured'));

-- +goose Down
ALTER TABLE workspace_codebases
    DROP CONSTRAINT workspace_codebases_preview_gateway_check;

ALTER TABLE workspace_codebases
    DROP COLUMN preview_gateway;
