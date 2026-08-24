-- +goose Up
DELETE FROM workspace_execution_previews;

ALTER TABLE workspace_execution_previews
    ADD COLUMN port integer NOT NULL;

ALTER TABLE workspace_execution_previews
    ADD CONSTRAINT workspace_execution_previews_port_check
        CHECK (port BETWEEN 1 AND 65535);

DROP INDEX workspace_execution_previews_name_key;

CREATE UNIQUE INDEX workspace_execution_previews_port_key
    ON workspace_execution_previews (execution_id, port);

DROP INDEX workspace_execution_previews_host_key;

CREATE UNIQUE INDEX workspace_execution_previews_host_key
    ON workspace_execution_previews (host) WHERE host <> '' AND mode = 'subdomain';

-- +goose Down
DELETE FROM workspace_execution_previews;

DROP INDEX workspace_execution_previews_host_key;

CREATE UNIQUE INDEX workspace_execution_previews_host_key
    ON workspace_execution_previews (host) WHERE host <> '';

DROP INDEX workspace_execution_previews_port_key;

CREATE UNIQUE INDEX workspace_execution_previews_name_key
    ON workspace_execution_previews (execution_id, name);

ALTER TABLE workspace_execution_previews
    DROP CONSTRAINT workspace_execution_previews_port_check;

ALTER TABLE workspace_execution_previews
    DROP COLUMN port;
