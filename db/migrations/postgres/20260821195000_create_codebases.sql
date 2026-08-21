-- +goose Up
CREATE TABLE workspace_codebases (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    runner_id       uuid NOT NULL REFERENCES workspace_runners (id) ON DELETE CASCADE,
    name            text NOT NULL,
    root_path       text NOT NULL,
    state           text NOT NULL DEFAULT 'active',
    shared_files    text[] NOT NULL DEFAULT '{}',
    runtimes        text[] NOT NULL DEFAULT '{}',
    tools           jsonb NOT NULL DEFAULT '[]',
    connected_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz,
    disconnected_at timestamptz,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_codebases_state_check
        CHECK (state IN ('active', 'drift', 'disconnected')),
    CONSTRAINT workspace_codebases_disconnected_check
        CHECK ((state = 'disconnected') = (disconnected_at IS NOT NULL)),
    CONSTRAINT workspace_codebases_name_check CHECK (name <> ''),
    CONSTRAINT workspace_codebases_root_check CHECK (root_path <> ''),
    CONSTRAINT workspace_codebases_tools_check CHECK (jsonb_typeof(tools) = 'array')
);

CREATE UNIQUE INDEX workspace_codebases_live_root_key
    ON workspace_codebases (runner_id, root_path) WHERE state <> 'disconnected';

CREATE INDEX workspace_codebases_runner_idx
    ON workspace_codebases (runner_id, connected_at DESC, id);

CREATE TABLE workspace_codebase_repositories (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    codebase_id      uuid NOT NULL REFERENCES workspace_codebases (id) ON DELETE CASCADE,
    ordinal          integer NOT NULL,
    name             text NOT NULL,
    rel_path         text NOT NULL,
    default_branch   text NOT NULL DEFAULT '',
    remote_hash      text NOT NULL DEFAULT '',
    remote_host      text NOT NULL DEFAULT '',
    remote_path_tail text NOT NULL DEFAULT '',
    CONSTRAINT workspace_codebase_repositories_name_check CHECK (name <> ''),
    CONSTRAINT workspace_codebase_repositories_path_check CHECK (rel_path <> ''),
    CONSTRAINT workspace_codebase_repositories_ordinal_check CHECK (ordinal >= 0)
);

CREATE UNIQUE INDEX workspace_codebase_repositories_path_key
    ON workspace_codebase_repositories (codebase_id, rel_path);

CREATE INDEX workspace_codebase_repositories_remote_idx
    ON workspace_codebase_repositories (remote_hash) WHERE remote_hash <> '';

-- +goose Down
DROP TABLE workspace_codebase_repositories;

DROP TABLE workspace_codebases;
