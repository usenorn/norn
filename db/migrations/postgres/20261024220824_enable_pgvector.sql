-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

-- +goose Down
DROP EXTENSION IF EXISTS vector;
