-- +goose Up
CREATE TABLE workspace_saved_views (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    created_by_account_id uuid,
    sharing               text NOT NULL DEFAULT 'personal',
    team_id               uuid,
    name                  text NOT NULL,
    filter                jsonb NOT NULL DEFAULT '{}',
    sort                  jsonb NOT NULL DEFAULT '[]',
    group_by              text NOT NULL DEFAULT '',
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_saved_views_name_check
        CHECK (name <> ''),
    CONSTRAINT workspace_saved_views_sharing_check
        CHECK (sharing IN ('personal', 'team', 'workspace')),
    CONSTRAINT workspace_saved_views_team_check
        CHECK ((sharing = 'team') = (team_id IS NOT NULL)),
    CONSTRAINT workspace_saved_views_group_by_check
        CHECK (group_by IN ('', 'state', 'stateCategory', 'priority', 'assignee',
                            'team', 'project', 'cycle', 'label')),
    CONSTRAINT workspace_saved_views_creator_fkey
        FOREIGN KEY (workspace_id, created_by_account_id)
        REFERENCES workspace_memberships (workspace_id, account_id)
        ON DELETE SET NULL (created_by_account_id),
    CONSTRAINT workspace_saved_views_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX workspace_saved_views_id_workspace_key
    ON workspace_saved_views (id, workspace_id);

CREATE INDEX workspace_saved_views_workspace_sharing_idx
    ON workspace_saved_views (workspace_id, sharing);

CREATE INDEX workspace_saved_views_workspace_creator_idx
    ON workspace_saved_views (workspace_id, created_by_account_id);

CREATE INDEX workspace_saved_views_team_idx
    ON workspace_saved_views (team_id) WHERE team_id IS NOT NULL;

CREATE TABLE workspace_saved_view_placements (
    workspace_id  uuid NOT NULL,
    account_id    uuid NOT NULL,
    saved_view_id uuid NOT NULL,
    position      integer NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, saved_view_id),
    CONSTRAINT workspace_saved_view_placements_position_check
        CHECK (position > 0),
    CONSTRAINT workspace_saved_view_placements_view_fkey
        FOREIGN KEY (saved_view_id, workspace_id)
        REFERENCES workspace_saved_views (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_saved_view_placements_membership_fkey
        FOREIGN KEY (workspace_id, account_id)
        REFERENCES workspace_memberships (workspace_id, account_id) ON DELETE CASCADE
);

CREATE INDEX workspace_saved_view_placements_account_position_idx
    ON workspace_saved_view_placements (workspace_id, account_id, position);

-- +goose Down
DROP TABLE workspace_saved_view_placements;

DROP TABLE workspace_saved_views;
