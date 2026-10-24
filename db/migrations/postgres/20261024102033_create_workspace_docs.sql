-- +goose Up
CREATE TABLE workspace_docs (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    parent_id         uuid,
    title             text NOT NULL,
    content           jsonb NOT NULL,
    markdown          text NOT NULL DEFAULT '',
    visibility        text NOT NULL,
    project_id        uuid,
    author_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    status            text NOT NULL DEFAULT 'active',
    archived_at       timestamptz,
    version           integer NOT NULL DEFAULT 1,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_docs_id_workspace_key UNIQUE (id, workspace_id),
    CONSTRAINT workspace_docs_parent_fkey FOREIGN KEY (parent_id, workspace_id)
        REFERENCES workspace_docs (id, workspace_id),
    CONSTRAINT workspace_docs_project_fkey FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id),
    CONSTRAINT workspace_docs_parent_check CHECK (parent_id IS DISTINCT FROM id),
    CONSTRAINT workspace_docs_title_check CHECK (char_length(title) BETWEEN 1 AND 200),
    CONSTRAINT workspace_docs_visibility_check CHECK (visibility IN ('personal', 'project', 'workspace')),
    CONSTRAINT workspace_docs_audience_check CHECK ((visibility = 'project') = (project_id IS NOT NULL)),
    CONSTRAINT workspace_docs_status_check CHECK (status IN ('active', 'archived')),
    CONSTRAINT workspace_docs_archived_check CHECK ((status = 'archived') = (archived_at IS NOT NULL)),
    CONSTRAINT workspace_docs_version_check CHECK (version > 0)
);

CREATE INDEX workspace_docs_listing_idx ON workspace_docs (workspace_id, updated_at DESC, id DESC)
    WHERE status = 'active';

CREATE INDEX workspace_docs_parent_idx ON workspace_docs (parent_id) WHERE parent_id IS NOT NULL;

CREATE INDEX workspace_docs_project_idx ON workspace_docs (project_id) WHERE project_id IS NOT NULL;

CREATE TABLE workspace_doc_revisions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  uuid NOT NULL,
    doc_id        uuid NOT NULL,
    version       integer NOT NULL,
    action        text NOT NULL,
    title         text NOT NULL,
    content       jsonb NOT NULL,
    markdown      text NOT NULL DEFAULT '',
    parent_id     uuid,
    visibility    text NOT NULL,
    project_id    uuid,
    account_id    uuid REFERENCES accounts (id) ON DELETE SET NULL,
    agent_id      uuid,
    connection_id uuid,
    source        text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_doc_revisions_doc_fkey FOREIGN KEY (doc_id, workspace_id)
        REFERENCES workspace_docs (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_doc_revisions_version_key UNIQUE (doc_id, version),
    CONSTRAINT workspace_doc_revisions_version_check CHECK (version > 0),
    CONSTRAINT workspace_doc_revisions_action_check CHECK (
        action IN ('created', 'edited', 'moved', 'reshared', 'restored', 'archived')
    ),
    CONSTRAINT workspace_doc_revisions_visibility_check CHECK (visibility IN ('personal', 'project', 'workspace')),
    CONSTRAINT workspace_doc_revisions_source_check CHECK (
        source IN ('person', 'agent', 'import', 'intake', 'restore')
    )
);

CREATE INDEX workspace_doc_revisions_account_idx ON workspace_doc_revisions (doc_id, account_id);

-- +goose Down
DROP TABLE IF EXISTS workspace_doc_revisions;
DROP TABLE IF EXISTS workspace_docs;
