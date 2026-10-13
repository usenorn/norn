-- +goose Up
CREATE TABLE workspace_roadmaps (
    id                                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id                       uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    title                              text NOT NULL DEFAULT '',
    description                        text NOT NULL DEFAULT '',
    show_issue_lists                   boolean NOT NULL DEFAULT false,
    show_project_descriptions          boolean NOT NULL DEFAULT false,
    show_issue_descriptions            boolean NOT NULL DEFAULT false,
    show_status                        boolean NOT NULL DEFAULT false,
    show_timeline                      boolean NOT NULL DEFAULT false,
    show_progress                      boolean NOT NULL DEFAULT false,
    version                            integer NOT NULL DEFAULT 1,
    published_version                  integer,
    published_title                    text,
    published_description              text,
    published_show_issue_lists         boolean,
    published_show_project_descriptions boolean,
    published_show_issue_descriptions  boolean,
    published_show_status              boolean,
    published_show_timeline            boolean,
    published_show_progress            boolean,
    published_at                       timestamptz,
    published_by_account_id            uuid REFERENCES accounts (id) ON DELETE SET NULL,
    created_at                         timestamptz NOT NULL DEFAULT now(),
    updated_at                         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_roadmaps_version_check CHECK (version > 0),
    CONSTRAINT workspace_roadmaps_title_check CHECK (char_length(title) <= 120),
    CONSTRAINT workspace_roadmaps_description_check CHECK (char_length(description) <= 4000),
    CONSTRAINT workspace_roadmaps_publication_check CHECK (
        (published_version IS NULL) = (published_at IS NULL)
        AND (published_version IS NULL) = (published_title IS NULL)
        AND (published_version IS NULL) = (published_description IS NULL)
        AND (published_version IS NULL) = (published_show_issue_lists IS NULL)
        AND (published_version IS NULL) = (published_show_project_descriptions IS NULL)
        AND (published_version IS NULL) = (published_show_issue_descriptions IS NULL)
        AND (published_version IS NULL) = (published_show_status IS NULL)
        AND (published_version IS NULL) = (published_show_timeline IS NULL)
        AND (published_version IS NULL) = (published_show_progress IS NULL)
    )
);

CREATE UNIQUE INDEX workspace_roadmaps_workspace_key ON workspace_roadmaps (workspace_id);

CREATE UNIQUE INDEX workspace_roadmaps_id_workspace_key ON workspace_roadmaps (id, workspace_id);

CREATE TABLE workspace_roadmap_entries (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id            uuid NOT NULL,
    workspace_id          uuid NOT NULL,
    public_id             text NOT NULL,
    kind                  text NOT NULL,
    source_project_id     uuid,
    source_issue_id       uuid,
    parent_entry_id       uuid REFERENCES workspace_roadmap_entries (id) ON DELETE SET NULL,
    public_title          text NOT NULL,
    public_description    text NOT NULL DEFAULT '',
    status                text NOT NULL,
    start_on              date,
    end_on                date,
    progress_mode         text NOT NULL DEFAULT 'hidden',
    progress_percent      integer NOT NULL DEFAULT 0,
    show_issue_lists          boolean NOT NULL DEFAULT true,
    show_project_descriptions boolean NOT NULL DEFAULT true,
    show_issue_descriptions   boolean NOT NULL DEFAULT true,
    show_status               boolean NOT NULL DEFAULT true,
    show_timeline             boolean NOT NULL DEFAULT true,
    show_progress             boolean NOT NULL DEFAULT true,
    position              integer NOT NULL DEFAULT 0,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_roadmap_entries_roadmap_fkey
        FOREIGN KEY (roadmap_id, workspace_id)
        REFERENCES workspace_roadmaps (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_roadmap_entries_project_fkey
        FOREIGN KEY (source_project_id, workspace_id)
        REFERENCES workspace_projects (id, workspace_id) ON DELETE SET NULL (source_project_id),
    CONSTRAINT workspace_roadmap_entries_issue_fkey
        FOREIGN KEY (source_issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE SET NULL (source_issue_id),
    CONSTRAINT workspace_roadmap_entries_kind_check CHECK (
        (kind = 'project' AND source_issue_id IS NULL)
        OR (kind = 'issue' AND source_project_id IS NULL)
    ),
    CONSTRAINT workspace_roadmap_entries_public_id_check CHECK (public_id ~ '^[a-z2-7]{16}$'),
    CONSTRAINT workspace_roadmap_entries_title_check
        CHECK (btrim(public_title) <> '' AND char_length(public_title) <= 120),
    CONSTRAINT workspace_roadmap_entries_description_check CHECK (char_length(public_description) <= 4000),
    CONSTRAINT workspace_roadmap_entries_status_check
        CHECK (status IN ('planned', 'in_progress', 'paused', 'completed', 'cancelled')),
    CONSTRAINT workspace_roadmap_entries_timeline_check
        CHECK (start_on IS NULL OR (end_on IS NOT NULL AND end_on >= start_on)),
    CONSTRAINT workspace_roadmap_entries_progress_mode_check
        CHECK (progress_mode IN ('hidden', 'approved', 'calculated')),
    CONSTRAINT workspace_roadmap_entries_progress_percent_check
        CHECK (progress_percent BETWEEN 0 AND 100)
);

CREATE UNIQUE INDEX workspace_roadmap_entries_public_id_key ON workspace_roadmap_entries (public_id);

CREATE UNIQUE INDEX workspace_roadmap_entries_project_key
    ON workspace_roadmap_entries (roadmap_id, source_project_id);

CREATE UNIQUE INDEX workspace_roadmap_entries_issue_key
    ON workspace_roadmap_entries (roadmap_id, source_issue_id);

CREATE INDEX workspace_roadmap_entries_roadmap_position_idx
    ON workspace_roadmap_entries (roadmap_id, position);

CREATE TABLE workspace_roadmap_published_entries (
    roadmap_id                uuid NOT NULL,
    workspace_id              uuid NOT NULL,
    public_id                 text NOT NULL,
    kind                      text NOT NULL,
    source_project_id         uuid,
    source_issue_id           uuid,
    parent_public_id          text,
    public_title              text NOT NULL,
    public_description        text NOT NULL,
    status                    text NOT NULL,
    start_on                  date,
    end_on                    date,
    progress_mode             text NOT NULL,
    progress_percent          integer NOT NULL,
    show_issue_lists          boolean NOT NULL,
    show_project_descriptions boolean NOT NULL,
    show_issue_descriptions   boolean NOT NULL,
    show_status               boolean NOT NULL,
    show_timeline             boolean NOT NULL,
    show_progress             boolean NOT NULL,
    position                  integer NOT NULL,
    approved_team_id          uuid,
    approved_team_private     boolean NOT NULL DEFAULT false,
    approved_project_id       uuid,
    PRIMARY KEY (roadmap_id, public_id),
    CONSTRAINT workspace_roadmap_published_entries_roadmap_fkey
        FOREIGN KEY (roadmap_id, workspace_id)
        REFERENCES workspace_roadmaps (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_roadmap_published_entries_kind_check CHECK (
        (kind = 'project' AND source_project_id IS NOT NULL AND source_issue_id IS NULL)
        OR (kind = 'issue' AND source_issue_id IS NOT NULL AND source_project_id IS NULL)
    ),
    CONSTRAINT workspace_roadmap_published_entries_title_check CHECK (btrim(public_title) <> '')
);

-- +goose Down
DROP TABLE workspace_roadmap_published_entries;
DROP TABLE workspace_roadmap_entries;
DROP TABLE workspace_roadmaps;
