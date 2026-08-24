-- +goose Up
ALTER TABLE workspace_execution_previews
    ADD COLUMN port integer NOT NULL DEFAULT 0;

ALTER TABLE workspace_execution_previews
    ADD CONSTRAINT workspace_execution_previews_port_check
        CHECK (port >= 0 AND port <= 65535);

DROP INDEX workspace_execution_previews_name_key;

CREATE UNIQUE INDEX workspace_execution_previews_port_key
    ON workspace_execution_previews (execution_id, port) WHERE port > 0;

-- +goose Down
DROP INDEX workspace_execution_previews_port_key;

CREATE UNIQUE INDEX workspace_execution_previews_name_key
    ON workspace_execution_previews (execution_id, name);

ALTER TABLE workspace_execution_previews
    DROP CONSTRAINT workspace_execution_previews_port_check;

ALTER TABLE workspace_execution_previews
    DROP COLUMN port;
