-- +goose Up
CREATE TABLE workspace_execution_snapshots (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id text NOT NULL REFERENCES workspace_executions (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    revision     integer NOT NULL,
    summary      text NOT NULL DEFAULT '',
    reported_at  timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_execution_snapshots_revision_check CHECK (revision >= 1)
);

CREATE UNIQUE INDEX workspace_execution_snapshots_revision_key
    ON workspace_execution_snapshots (execution_id, revision);

CREATE TABLE workspace_execution_snapshot_repositories (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id      uuid NOT NULL REFERENCES workspace_execution_snapshots (id) ON DELETE CASCADE,
    repository       text NOT NULL,
    branch           text NOT NULL DEFAULT '',
    base_sha         text NOT NULL DEFAULT '',
    head_sha         text NOT NULL DEFAULT '',
    commits          jsonb NOT NULL DEFAULT '[]',
    additions        integer NOT NULL DEFAULT 0,
    deletions        integer NOT NULL DEFAULT 0,
    files_changed    integer NOT NULL DEFAULT 0,
    diff_artifact_id uuid REFERENCES workspace_execution_artifacts (id) ON DELETE SET NULL,
    CONSTRAINT workspace_execution_snapshot_repositories_repository_check CHECK (repository <> ''),
    CONSTRAINT workspace_execution_snapshot_repositories_commits_check
        CHECK (jsonb_typeof(commits) = 'array'),
    CONSTRAINT workspace_execution_snapshot_repositories_counts_check
        CHECK (additions >= 0 AND deletions >= 0 AND files_changed >= 0)
);

CREATE UNIQUE INDEX workspace_execution_snapshot_repositories_key
    ON workspace_execution_snapshot_repositories (snapshot_id, repository);

CREATE TABLE workspace_execution_snapshot_previews (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id uuid NOT NULL REFERENCES workspace_execution_snapshots (id) ON DELETE CASCADE,
    name        text NOT NULL,
    service     text NOT NULL,
    path        text NOT NULL DEFAULT '',
    state       text NOT NULL,
    reason      text NOT NULL DEFAULT '',
    port        integer NOT NULL DEFAULT 0,
    CONSTRAINT workspace_execution_snapshot_previews_name_check CHECK (name <> '' AND service <> ''),
    CONSTRAINT workspace_execution_snapshot_previews_state_check
        CHECK (state IN ('ready', 'failed', 'unsupported')),
    CONSTRAINT workspace_execution_snapshot_previews_port_check CHECK (port >= 0)
);

CREATE UNIQUE INDEX workspace_execution_snapshot_previews_key
    ON workspace_execution_snapshot_previews (snapshot_id, name);

ALTER TABLE workspace_execution_review_comments
    ADD COLUMN revision integer NOT NULL DEFAULT 1,
    ADD CONSTRAINT workspace_execution_review_comments_revision_check CHECK (revision >= 1);

ALTER TABLE workspace_execution_review_comments ALTER COLUMN revision DROP DEFAULT;

ALTER TABLE workspace_execution_reviews
    ADD COLUMN revision integer NOT NULL DEFAULT 1,
    ADD CONSTRAINT workspace_execution_reviews_revision_check CHECK (revision >= 1);

ALTER TABLE workspace_execution_reviews ALTER COLUMN revision DROP DEFAULT;

-- +goose Down
ALTER TABLE workspace_execution_reviews DROP COLUMN revision;

ALTER TABLE workspace_execution_review_comments DROP COLUMN revision;

DROP TABLE workspace_execution_snapshot_previews;

DROP TABLE workspace_execution_snapshot_repositories;

DROP TABLE workspace_execution_snapshots;
