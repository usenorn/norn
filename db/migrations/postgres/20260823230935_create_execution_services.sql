-- +goose Up
CREATE TABLE workspace_execution_services (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    state        text NOT NULL,
    probe        text NOT NULL DEFAULT '',
    port         integer NOT NULL DEFAULT 0,
    reason       text NOT NULL DEFAULT '',
    reported_at  timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_services_name_check CHECK (name <> ''),
    CONSTRAINT workspace_execution_services_state_check
        CHECK (state IN ('starting', 'healthy', 'unhealthy', 'stopped')),
    CONSTRAINT workspace_execution_services_probe_check
        CHECK (probe IN ('', 'http', 'tcp', 'log')),
    CONSTRAINT workspace_execution_services_port_check
        CHECK (port >= 0 AND port <= 65535)
);

CREATE UNIQUE INDEX workspace_execution_services_name_key
    ON workspace_execution_services (execution_id, name);

CREATE INDEX workspace_execution_services_execution_idx
    ON workspace_execution_services (execution_id, name, id);

-- +goose Down
DROP TABLE workspace_execution_services;
