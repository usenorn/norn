-- +goose Up
ALTER TABLE accounts
    ADD COLUMN instance_admin boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE accounts
    DROP COLUMN instance_admin;
