-- +goose Up
ALTER TABLE workspace_ai_providers
    ADD COLUMN embedding_model text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE workspace_ai_providers
    DROP COLUMN embedding_model;
