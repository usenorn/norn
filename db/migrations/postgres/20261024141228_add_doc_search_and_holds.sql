-- +goose Up
ALTER TABLE workspace_docs
    ADD COLUMN search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', title), 'A') ||
            setweight(to_tsvector('simple', title), 'A') ||
            setweight(to_tsvector('english', markdown), 'B') ||
            setweight(to_tsvector('simple', markdown), 'B')
        ) STORED;

CREATE INDEX workspace_docs_search_idx
    ON workspace_docs USING gin (workspace_id, search_document)
    WHERE status = 'active';

CREATE TABLE workspace_agent_settings (
    workspace_id    uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    hold_doc_writes text NOT NULL DEFAULT 'never',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_agent_settings_doc_writes_check
        CHECK (hold_doc_writes IN ('never', 'always'))
);

ALTER TABLE workspace_agent_proposals
    ALTER COLUMN team_id DROP NOT NULL,
    ADD COLUMN doc_id uuid,
    ADD CONSTRAINT workspace_agent_proposals_doc_fkey FOREIGN KEY (doc_id, workspace_id)
        REFERENCES workspace_docs (id, workspace_id) ON DELETE CASCADE,
    DROP CONSTRAINT workspace_agent_proposals_issue_check,
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'issue_create', 'doc_create', 'doc_edit')),
    ADD CONSTRAINT workspace_agent_proposals_subject_check
        CHECK (CASE action
            WHEN 'issue_create' THEN issue_id IS NULL AND team_id IS NOT NULL AND doc_id IS NULL
            WHEN 'doc_create' THEN issue_id IS NULL AND team_id IS NULL AND doc_id IS NULL
            WHEN 'doc_edit' THEN issue_id IS NULL AND team_id IS NULL AND doc_id IS NOT NULL
            ELSE issue_id IS NOT NULL AND team_id IS NOT NULL AND doc_id IS NULL
        END);

CREATE INDEX workspace_agent_proposals_doc_idx
    ON workspace_agent_proposals (doc_id) WHERE doc_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS workspace_agent_proposals_doc_idx;

DELETE FROM workspace_agent_proposals WHERE action IN ('doc_create', 'doc_edit');

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_subject_check,
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'issue_create')),
    ADD CONSTRAINT workspace_agent_proposals_issue_check
        CHECK ((action = 'issue_create') = (issue_id IS NULL)),
    DROP CONSTRAINT workspace_agent_proposals_doc_fkey,
    DROP COLUMN doc_id,
    ALTER COLUMN team_id SET NOT NULL;

DROP TABLE IF EXISTS workspace_agent_settings;

DROP INDEX IF EXISTS workspace_docs_search_idx;

ALTER TABLE workspace_docs DROP COLUMN search_document;
