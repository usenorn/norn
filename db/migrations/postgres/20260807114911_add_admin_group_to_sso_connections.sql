-- +goose Up
ALTER TABLE workspace_sso_connections
    ADD COLUMN admin_group text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE workspace_sso_connections
    DROP COLUMN admin_group;
