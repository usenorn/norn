-- +goose Up
ALTER TABLE workspace_execution_previews
    ADD COLUMN port integer NOT NULL DEFAULT 0;

ALTER TABLE workspace_execution_previews
    ADD CONSTRAINT workspace_execution_previews_port_check
        CHECK (port >= 0 AND port <= 65535);

DROP INDEX workspace_execution_previews_host_key;

CREATE INDEX workspace_execution_previews_host_idx
    ON workspace_execution_previews (host) WHERE host <> '';

UPDATE workspace_execution_previews SET host = '' WHERE host <> '';

-- +goose Down
UPDATE workspace_execution_previews SET host = '' WHERE host <> '';

DROP INDEX workspace_execution_previews_host_idx;

CREATE UNIQUE INDEX workspace_execution_previews_host_key
    ON workspace_execution_previews (host) WHERE host <> '';

ALTER TABLE workspace_execution_previews
    DROP CONSTRAINT workspace_execution_previews_port_check;

ALTER TABLE workspace_execution_previews
    DROP COLUMN port;
