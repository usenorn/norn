-- +goose Up
ALTER TABLE workspace_code_links
    ADD COLUMN merge_commit_sha text NOT NULL DEFAULT '';

CREATE INDEX workspace_code_links_merge_commit_idx
    ON workspace_code_links (repository_id, merge_commit_sha)
    WHERE merge_commit_sha <> '';

ALTER TABLE workspace_scm_repositories
    ADD COLUMN backfilled_at timestamptz;

CREATE TABLE workspace_scm_releases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id uuid NOT NULL REFERENCES workspace_scm_repositories (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    external_id text NOT NULL,
    tag text NOT NULL,
    name text NOT NULL DEFAULT '',
    url text NOT NULL DEFAULT '',
    commit_sha text NOT NULL DEFAULT '',
    prerelease boolean NOT NULL DEFAULT false,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_scm_releases_tag_check CHECK (tag <> '')
);

CREATE UNIQUE INDEX workspace_scm_releases_external_key
    ON workspace_scm_releases (repository_id, external_id);

CREATE INDEX workspace_scm_releases_recent_idx
    ON workspace_scm_releases (repository_id, published_at DESC NULLS LAST, id);

CREATE TABLE workspace_scm_release_links (
    release_id uuid NOT NULL REFERENCES workspace_scm_releases (id) ON DELETE CASCADE,
    link_id uuid NOT NULL REFERENCES workspace_code_links (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL REFERENCES workspace_issues (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (release_id, link_id)
);

CREATE INDEX workspace_scm_release_links_issue_idx
    ON workspace_scm_release_links (issue_id, created_at DESC);

CREATE TABLE workspace_scm_deployments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id uuid NOT NULL REFERENCES workspace_scm_repositories (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    external_id text NOT NULL,
    environment text NOT NULL,
    state text NOT NULL,
    url text NOT NULL DEFAULT '',
    commit_sha text NOT NULL DEFAULT '',
    occurred_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_scm_deployments_environment_check CHECK (environment <> ''),
    CONSTRAINT workspace_scm_deployments_state_check
        CHECK (state IN ('pending', 'running', 'succeeded', 'failed', 'inactive'))
);

CREATE UNIQUE INDEX workspace_scm_deployments_external_key
    ON workspace_scm_deployments (repository_id, external_id);

CREATE INDEX workspace_scm_deployments_commit_idx
    ON workspace_scm_deployments (repository_id, commit_sha, occurred_at DESC);

-- +goose Down
DROP TABLE IF EXISTS workspace_scm_deployments;

DROP TABLE IF EXISTS workspace_scm_release_links;

DROP TABLE IF EXISTS workspace_scm_releases;

ALTER TABLE workspace_scm_repositories
    DROP COLUMN backfilled_at;

DROP INDEX IF EXISTS workspace_code_links_merge_commit_idx;

ALTER TABLE workspace_code_links
    DROP COLUMN merge_commit_sha;
