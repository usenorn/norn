-- +goose Up
ALTER TABLE workspaces
    ADD COLUMN logo_object_key text,
    ADD COLUMN week_starts_on  text NOT NULL DEFAULT 'monday';

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_week_starts_on_check
        CHECK (week_starts_on IN ('monday', 'sunday'));

-- +goose Down
ALTER TABLE workspaces
    DROP CONSTRAINT workspaces_week_starts_on_check;

ALTER TABLE workspaces
    DROP COLUMN week_starts_on,
    DROP COLUMN logo_object_key;
