-- +goose Up
CREATE TABLE workspace_doc_passages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    uuid NOT NULL,
    doc_id          uuid NOT NULL,
    doc_version     integer NOT NULL,
    position        integer NOT NULL,
    heading         text NOT NULL,
    body            text NOT NULL,
    search_document tsvector
        GENERATED ALWAYS AS (
            setweight(to_tsvector('english', heading), 'A') ||
            setweight(to_tsvector('simple', heading), 'A') ||
            setweight(to_tsvector('english', body), 'B') ||
            setweight(to_tsvector('simple', body), 'B')
        ) STORED,
    content_hash    bytea NOT NULL
        GENERATED ALWAYS AS (decode(md5(heading || E'\n' || body), 'hex')) STORED,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_doc_passages_doc_fkey FOREIGN KEY (doc_id, workspace_id)
        REFERENCES workspace_docs (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_doc_passages_position_key UNIQUE (doc_id, position),
    CONSTRAINT workspace_doc_passages_position_check CHECK (position >= 0),
    CONSTRAINT workspace_doc_passages_version_check CHECK (doc_version > 0),
    CONSTRAINT workspace_doc_passages_text_check CHECK (char_length(heading) + char_length(body) > 0)
);

CREATE INDEX workspace_doc_passages_search_idx ON workspace_doc_passages USING gin (workspace_id, search_document);

CREATE INDEX workspace_doc_passages_hash_idx ON workspace_doc_passages (workspace_id, content_hash);

-- +goose Down
DROP TABLE workspace_doc_passages;
