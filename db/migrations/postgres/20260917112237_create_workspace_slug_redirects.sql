-- +goose Up
CREATE TABLE workspace_slug_redirects (
    slug         text PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workspace_slug_redirects_workspace_id_idx
    ON workspace_slug_redirects (workspace_id);

-- +goose Down
DROP TABLE workspace_slug_redirects;
