-- +goose Up
ALTER TABLE workspace_scm_repositories
    ADD COLUMN announce_description boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_scm_repositories
    DROP COLUMN announce_description;
