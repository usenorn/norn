-- +goose Up
CREATE TABLE workspace_execution_policies (
    workspace_id          uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    telemetry             text NOT NULL DEFAULT 'full',
    upload_retention_days integer NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_policies_telemetry_check
        CHECK (telemetry IN ('full', 'minimal')),
    CONSTRAINT workspace_execution_policies_retention_check
        CHECK (upload_retention_days BETWEEN 1 AND 3650)
);

CREATE TABLE workspace_execution_chunks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    stream       text NOT NULL,
    sequence     bigint NOT NULL,
    digest       text NOT NULL,
    bytes        bigint NOT NULL,
    entries      integer NOT NULL,
    object_key   text NOT NULL,
    first_at     timestamptz NOT NULL,
    last_at      timestamptz NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_chunks_stream_check
        CHECK (stream IN ('logs', 'transcript')),
    CONSTRAINT workspace_execution_chunks_sequence_check CHECK (sequence >= 1),
    CONSTRAINT workspace_execution_chunks_bytes_check CHECK (bytes > 0),
    CONSTRAINT workspace_execution_chunks_entries_check CHECK (entries > 0)
);

CREATE UNIQUE INDEX workspace_execution_chunks_digest_key
    ON workspace_execution_chunks (execution_id, stream, digest);

CREATE UNIQUE INDEX workspace_execution_chunks_sequence_key
    ON workspace_execution_chunks (execution_id, stream, sequence);

CREATE INDEX workspace_execution_chunks_received_idx
    ON workspace_execution_chunks (stream, received_at);

CREATE TABLE workspace_execution_artifacts (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    content_type text NOT NULL,
    bytes        bigint NOT NULL,
    digest       text NOT NULL,
    object_key   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_artifacts_bytes_check CHECK (bytes > 0)
);

CREATE UNIQUE INDEX workspace_execution_artifacts_digest_key
    ON workspace_execution_artifacts (execution_id, digest);

CREATE INDEX workspace_execution_artifacts_execution_idx
    ON workspace_execution_artifacts (execution_id, created_at, id);

-- +goose Down
DROP TABLE workspace_execution_artifacts;

DROP TABLE workspace_execution_chunks;

DROP TABLE workspace_execution_policies;
