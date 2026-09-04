-- +goose Up
ALTER TABLE workspace_teams
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN icon        text NOT NULL DEFAULT '',
    ADD COLUMN icon_color  text NOT NULL DEFAULT 'neutral',
    ADD COLUMN estimation  text NOT NULL DEFAULT 'none';

ALTER TABLE workspace_teams
    ADD CONSTRAINT workspace_teams_icon_color_check
        CHECK (icon_color IN ('neutral', 'cyan', 'blue', 'violet', 'orchid', 'magenta')),
    ADD CONSTRAINT workspace_teams_estimation_check
        CHECK (estimation IN ('none', 'points', 'hours', 'sizes'));

-- +goose Down
ALTER TABLE workspace_teams
    DROP CONSTRAINT workspace_teams_estimation_check,
    DROP CONSTRAINT workspace_teams_icon_color_check;

ALTER TABLE workspace_teams
    DROP COLUMN estimation,
    DROP COLUMN icon_color,
    DROP COLUMN icon,
    DROP COLUMN description;
