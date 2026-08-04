-- +goose Up
ALTER TABLE workspace_bulk_actions
    ADD COLUMN requested_actor_kind text NOT NULL DEFAULT 'user',
    ADD COLUMN requested_auth_method text NOT NULL DEFAULT '';

ALTER TABLE workspace_bulk_actions
    ADD CONSTRAINT workspace_bulk_actions_actor_kind_check
        CHECK (requested_actor_kind IN ('user', 'token', 'agent'));

-- +goose Down
ALTER TABLE workspace_bulk_actions
    DROP CONSTRAINT workspace_bulk_actions_actor_kind_check;

ALTER TABLE workspace_bulk_actions
    DROP COLUMN requested_auth_method,
    DROP COLUMN requested_actor_kind;
