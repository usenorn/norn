-- +goose Up
ALTER TABLE workspace_memories
    ADD COLUMN search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', statement), 'A') ||
            setweight(to_tsvector('simple', statement), 'A') ||
            setweight(to_tsvector('english', rationale || ' ' || conditions), 'B') ||
            setweight(to_tsvector('simple', rationale || ' ' || conditions), 'B')
        ) STORED,
    ADD COLUMN content_hash bytea NOT NULL
        GENERATED ALWAYS AS (
            decode(md5(statement || E'\n' || rationale || E'\n' || conditions), 'hex')
        ) STORED;

CREATE INDEX workspace_memories_search_idx ON workspace_memories USING gin (workspace_id, search_document);

-- +goose Down
DROP INDEX workspace_memories_search_idx;

ALTER TABLE workspace_memories
    DROP COLUMN content_hash,
    DROP COLUMN search_document;
