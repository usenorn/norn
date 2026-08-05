-- +goose Up
ALTER TABLE workspace_activity
    ADD COLUMN actor_token_id uuid REFERENCES api_tokens (id) ON DELETE SET NULL,
    ADD COLUMN actor_token_name text;

-- +goose Down
ALTER TABLE workspace_activity
    DROP COLUMN actor_token_name,
    DROP COLUMN actor_token_id;
