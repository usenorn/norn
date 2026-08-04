-- +goose Up
CREATE UNIQUE INDEX workspace_issues_id_workspace_key ON workspace_issues (id, workspace_id);
CREATE UNIQUE INDEX workspace_issues_id_team_key ON workspace_issues (id, team_id);

CREATE TABLE workspace_label_groups (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workspace_label_groups_workspace_name_key
    ON workspace_label_groups (workspace_id, lower(name));

CREATE UNIQUE INDEX workspace_label_groups_id_workspace_key
    ON workspace_label_groups (id, workspace_id);

CREATE TABLE workspace_labels (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    team_id      uuid,
    group_id     uuid,
    name         text NOT NULL,
    color        text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_labels_color_check
        CHECK (color IN ('neutral', 'cyan', 'blue', 'violet', 'orchid', 'magenta')),
    CONSTRAINT workspace_labels_team_fkey
        FOREIGN KEY (team_id, workspace_id)
        REFERENCES workspace_teams (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_labels_group_fkey
        FOREIGN KEY (group_id, workspace_id)
        REFERENCES workspace_label_groups (id, workspace_id)
);

CREATE UNIQUE INDEX workspace_labels_workspace_name_key
    ON workspace_labels (workspace_id, lower(name)) WHERE team_id IS NULL;

CREATE UNIQUE INDEX workspace_labels_team_name_key
    ON workspace_labels (team_id, lower(name)) WHERE team_id IS NOT NULL;

CREATE UNIQUE INDEX workspace_labels_id_workspace_key ON workspace_labels (id, workspace_id);
CREATE UNIQUE INDEX workspace_labels_id_team_key ON workspace_labels (id, team_id);
CREATE UNIQUE INDEX workspace_labels_id_group_key ON workspace_labels (id, group_id);
CREATE INDEX workspace_labels_group_idx ON workspace_labels (group_id);

CREATE TABLE workspace_issue_labels (
    workspace_id   uuid NOT NULL,
    issue_id       uuid NOT NULL,
    label_id       uuid NOT NULL,
    label_team_id  uuid,
    label_group_id uuid,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (issue_id, label_id),
    CONSTRAINT workspace_issue_labels_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_labels_label_fkey
        FOREIGN KEY (label_id, workspace_id)
        REFERENCES workspace_labels (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_labels_label_team_fkey
        FOREIGN KEY (label_id, label_team_id)
        REFERENCES workspace_labels (id, team_id) ON UPDATE CASCADE,
    CONSTRAINT workspace_issue_labels_issue_team_fkey
        FOREIGN KEY (issue_id, label_team_id)
        REFERENCES workspace_issues (id, team_id),
    CONSTRAINT workspace_issue_labels_label_group_fkey
        FOREIGN KEY (label_id, label_group_id)
        REFERENCES workspace_labels (id, group_id) ON UPDATE CASCADE
);

CREATE UNIQUE INDEX workspace_issue_labels_issue_group_key
    ON workspace_issue_labels (issue_id, label_group_id) WHERE label_group_id IS NOT NULL;

CREATE INDEX workspace_issue_labels_label_idx ON workspace_issue_labels (label_id);

-- +goose Down
DROP TABLE workspace_issue_labels;
DROP TABLE workspace_labels;
DROP TABLE workspace_label_groups;
DROP INDEX workspace_issues_id_team_key;
DROP INDEX workspace_issues_id_workspace_key;
