-- +goose Up
CREATE TABLE workspace_memories (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    kind               text NOT NULL,
    statement          text NOT NULL,
    rationale          text NOT NULL DEFAULT '',
    conditions         text NOT NULL DEFAULT '',
    visibility         text NOT NULL,
    project_id         uuid,
    applies_project_id uuid,
    component          text NOT NULL DEFAULT '',
    status             text NOT NULL,
    basis              text NOT NULL,
    author_account_id  uuid REFERENCES accounts (id) ON DELETE SET NULL,
    agent_id           uuid,
    connection_id      uuid,
    source             text NOT NULL,
    effective_from     date NOT NULL,
    effective_until    date,
    supersedes_id      uuid,
    superseded_by_id   uuid,
    version            integer NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_memories_id_workspace_key UNIQUE (id, workspace_id),
    CONSTRAINT workspace_memories_project_fkey FOREIGN KEY (project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id),
    CONSTRAINT workspace_memories_applies_project_fkey FOREIGN KEY (applies_project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id),
    CONSTRAINT workspace_memories_supersedes_fkey FOREIGN KEY (supersedes_id, workspace_id)
        REFERENCES workspace_memories (id, workspace_id),
    CONSTRAINT workspace_memories_superseded_by_fkey FOREIGN KEY (superseded_by_id, workspace_id)
        REFERENCES workspace_memories (id, workspace_id),
    CONSTRAINT workspace_memories_kind_check CHECK (kind IN ('decision', 'constraint', 'lesson', 'preference')),
    CONSTRAINT workspace_memories_statement_check CHECK (char_length(statement) BETWEEN 1 AND 500),
    CONSTRAINT workspace_memories_component_check CHECK (char_length(component) <= 100),
    CONSTRAINT workspace_memories_visibility_check CHECK (visibility IN ('personal', 'project', 'workspace')),
    CONSTRAINT workspace_memories_audience_check CHECK ((visibility = 'project') = (project_id IS NOT NULL)),
    CONSTRAINT workspace_memories_status_check CHECK (
        status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn')
    ),
    CONSTRAINT workspace_memories_basis_check CHECK (basis IN ('observation', 'inference')),
    CONSTRAINT workspace_memories_source_check CHECK (
        source IN ('person', 'agent', 'import', 'intake', 'restore')
    ),
    CONSTRAINT workspace_memories_effective_check CHECK (effective_until IS NULL OR effective_until >= effective_from),
    CONSTRAINT workspace_memories_supersedes_check CHECK (supersedes_id IS DISTINCT FROM id),
    CONSTRAINT workspace_memories_superseded_check CHECK ((status = 'superseded') = (superseded_by_id IS NOT NULL)),
    CONSTRAINT workspace_memories_version_check CHECK (version > 0)
);

CREATE INDEX workspace_memories_listing_idx ON workspace_memories (workspace_id, updated_at DESC, id DESC);

CREATE INDEX workspace_memories_project_idx ON workspace_memories (project_id) WHERE project_id IS NOT NULL;

CREATE INDEX workspace_memories_applies_project_idx ON workspace_memories (applies_project_id)
    WHERE applies_project_id IS NOT NULL;

CREATE INDEX workspace_memories_supersedes_idx ON workspace_memories (supersedes_id) WHERE supersedes_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_doc_revisions_id_doc_workspace_key
    ON workspace_doc_revisions (id, doc_id, workspace_id);

CREATE TABLE workspace_memory_evidence (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    uuid NOT NULL,
    memory_id       uuid NOT NULL,
    position        integer NOT NULL,
    kind            text NOT NULL,
    issue_id        uuid,
    comment_id      uuid,
    doc_id          uuid,
    doc_revision_id uuid,
    excerpt         text,
    CONSTRAINT workspace_memory_evidence_memory_fkey FOREIGN KEY (memory_id, workspace_id)
        REFERENCES workspace_memories (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_memory_evidence_issue_fkey FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_memory_evidence_comment_fkey FOREIGN KEY (comment_id, workspace_id)
        REFERENCES workspace_issue_comments (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_memory_evidence_doc_revision_fkey FOREIGN KEY (doc_revision_id, doc_id, workspace_id)
        REFERENCES workspace_doc_revisions (id, doc_id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_memory_evidence_position_key UNIQUE (memory_id, position),
    CONSTRAINT workspace_memory_evidence_kind_check CHECK (
        CASE kind
            WHEN 'issue' THEN issue_id IS NOT NULL AND num_nonnulls(comment_id, doc_id, doc_revision_id, excerpt) = 0
            WHEN 'comment' THEN issue_id IS NOT NULL AND comment_id IS NOT NULL
                AND num_nonnulls(doc_id, doc_revision_id, excerpt) = 0
            WHEN 'doc_revision' THEN doc_id IS NOT NULL AND doc_revision_id IS NOT NULL
                AND num_nonnulls(issue_id, comment_id, excerpt) = 0
            WHEN 'excerpt' THEN excerpt IS NOT NULL AND num_nonnulls(issue_id, comment_id, doc_id, doc_revision_id) = 0
            ELSE false
        END
    ),
    CONSTRAINT workspace_memory_evidence_excerpt_check CHECK (char_length(excerpt) BETWEEN 1 AND 2000)
);

CREATE INDEX workspace_memory_evidence_issue_idx ON workspace_memory_evidence (issue_id) WHERE issue_id IS NOT NULL;

CREATE INDEX workspace_memory_evidence_comment_idx ON workspace_memory_evidence (comment_id)
    WHERE comment_id IS NOT NULL;

CREATE INDEX workspace_memory_evidence_doc_revision_idx ON workspace_memory_evidence (doc_revision_id)
    WHERE doc_revision_id IS NOT NULL;

CREATE TABLE workspace_memory_revisions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       uuid NOT NULL,
    memory_id          uuid NOT NULL,
    version            integer NOT NULL,
    action             text NOT NULL,
    kind               text NOT NULL,
    statement          text NOT NULL,
    rationale          text NOT NULL DEFAULT '',
    conditions         text NOT NULL DEFAULT '',
    visibility         text NOT NULL,
    project_id         uuid,
    applies_project_id uuid,
    component          text NOT NULL DEFAULT '',
    status             text NOT NULL,
    basis              text NOT NULL,
    evidence           jsonb NOT NULL DEFAULT '[]',
    effective_from     date NOT NULL,
    effective_until    date,
    account_id         uuid REFERENCES accounts (id) ON DELETE SET NULL,
    agent_id           uuid,
    connection_id      uuid,
    source             text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_memory_revisions_memory_fkey FOREIGN KEY (memory_id, workspace_id)
        REFERENCES workspace_memories (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_memory_revisions_version_key UNIQUE (memory_id, version),
    CONSTRAINT workspace_memory_revisions_version_check CHECK (version > 0),
    CONSTRAINT workspace_memory_revisions_action_check CHECK (
        action IN ('created', 'corrected', 'reshared', 'accepted', 'disputed', 'superseded', 'withdrawn')
    ),
    CONSTRAINT workspace_memory_revisions_status_check CHECK (
        status IN ('proposed', 'accepted', 'disputed', 'superseded', 'withdrawn')
    ),
    CONSTRAINT workspace_memory_revisions_visibility_check CHECK (
        visibility IN ('personal', 'project', 'workspace')
    ),
    CONSTRAINT workspace_memory_revisions_source_check CHECK (
        source IN ('person', 'agent', 'import', 'intake', 'restore')
    )
);

CREATE INDEX workspace_memory_revisions_account_idx ON workspace_memory_revisions (memory_id, account_id);

-- +goose Down
DROP TABLE IF EXISTS workspace_memory_revisions;
DROP TABLE IF EXISTS workspace_memory_evidence;
DROP INDEX IF EXISTS workspace_doc_revisions_id_doc_workspace_key;
DROP TABLE IF EXISTS workspace_memories;
