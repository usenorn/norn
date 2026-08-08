-- +goose Up
ALTER TABLE scm_apps
    ADD COLUMN allow_private_address boolean NOT NULL DEFAULT false,
    ADD COLUMN ca_certificate text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE scm_apps
    DROP COLUMN ca_certificate,
    DROP COLUMN allow_private_address;
