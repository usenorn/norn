-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE EXTENSION IF NOT EXISTS btree_gin;

ALTER TABLE workspace_issues
    ADD COLUMN search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', title), 'A') ||
            setweight(to_tsvector('simple', title), 'A') ||
            setweight(to_tsvector('english', description), 'B') ||
            setweight(to_tsvector('simple', description), 'B')
        ) STORED;

ALTER TABLE workspace_issue_comments
    ADD COLUMN search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', body), 'A') ||
            setweight(to_tsvector('simple', body), 'A')
        ) STORED;

ALTER TABLE workspace_projects
    ADD COLUMN search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', name), 'A') ||
            setweight(to_tsvector('simple', name), 'A') ||
            setweight(to_tsvector('english', description), 'B') ||
            setweight(to_tsvector('simple', description), 'B')
        ) STORED;

-- +goose Down
ALTER TABLE workspace_projects DROP COLUMN search_document;

ALTER TABLE workspace_issue_comments DROP COLUMN search_document;

ALTER TABLE workspace_issues DROP COLUMN search_document;
