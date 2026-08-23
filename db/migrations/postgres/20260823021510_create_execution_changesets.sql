-- +goose Up
CREATE TABLE workspace_execution_results (
    execution_id text PRIMARY KEY REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    summary      text NOT NULL DEFAULT '',
    reported_at  timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_execution_changes (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id     text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    repository       text NOT NULL,
    branch           text NOT NULL DEFAULT '',
    base_sha         text NOT NULL DEFAULT '',
    head_sha         text NOT NULL DEFAULT '',
    commits          integer NOT NULL DEFAULT 0,
    additions        integer NOT NULL DEFAULT 0,
    deletions        integer NOT NULL DEFAULT 0,
    files_changed    integer NOT NULL DEFAULT 0,
    diff_artifact_id uuid REFERENCES workspace_execution_artifacts (id) ON DELETE SET NULL,
    pull_request_url text NOT NULL DEFAULT '',
    code_link_id     uuid REFERENCES workspace_code_links (id) ON DELETE SET NULL,
    reported_at      timestamptz NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_changes_repository_check CHECK (repository <> ''),
    CONSTRAINT workspace_execution_changes_counts_check
        CHECK (commits >= 0 AND additions >= 0 AND deletions >= 0 AND files_changed >= 0)
);

CREATE UNIQUE INDEX workspace_execution_changes_repository_key
    ON workspace_execution_changes (execution_id, repository);

CREATE INDEX workspace_execution_changes_execution_idx
    ON workspace_execution_changes (execution_id, repository, id);

CREATE INDEX workspace_execution_changes_link_idx
    ON workspace_execution_changes (code_link_id) WHERE code_link_id IS NOT NULL;

CREATE TABLE workspace_execution_validations (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    check_name   text NOT NULL,
    status       text NOT NULL,
    detail       text NOT NULL DEFAULT '',
    artifact_id  uuid REFERENCES workspace_execution_artifacts (id) ON DELETE SET NULL,
    reported_at  timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_validations_name_check CHECK (check_name <> ''),
    CONSTRAINT workspace_execution_validations_status_check
        CHECK (status IN ('passed', 'failed', 'skipped'))
);

CREATE UNIQUE INDEX workspace_execution_validations_name_key
    ON workspace_execution_validations (execution_id, check_name);

CREATE INDEX workspace_execution_validations_execution_idx
    ON workspace_execution_validations (execution_id, check_name, id);

-- +goose Down
DROP TABLE workspace_execution_validations;

DROP TABLE workspace_execution_changes;

DROP TABLE workspace_execution_results;
