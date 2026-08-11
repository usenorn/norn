-- +goose Up
ALTER TABLE workspace_check_evidence DROP COLUMN actor_name;

-- +goose Down
ALTER TABLE workspace_check_evidence ADD COLUMN actor_name text NOT NULL DEFAULT '';
