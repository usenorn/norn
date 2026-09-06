-- +goose Up
CREATE TABLE workspace_project_links (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES workspace_projects (id) ON DELETE CASCADE,
    label      text NOT NULL,
    url        text NOT NULL,
    position   integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_project_links_label_check CHECK (label <> ''),
    CONSTRAINT workspace_project_links_url_check
        CHECK (url ~ '^https?://'),
    CONSTRAINT workspace_project_links_position_check CHECK (position >= 0)
);

CREATE INDEX workspace_project_links_project_idx
    ON workspace_project_links (project_id, position, id);

-- +goose Down
DROP TABLE workspace_project_links;
