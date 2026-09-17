-- +goose Up
ALTER TABLE workspace_labels
    ADD COLUMN description text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE workspace_labels
    DROP COLUMN description;
