-- +goose Up
ALTER TABLE workspace_code_links
    ADD COLUMN announced_at timestamptz;

-- +goose Down
ALTER TABLE workspace_code_links
    DROP COLUMN announced_at;
