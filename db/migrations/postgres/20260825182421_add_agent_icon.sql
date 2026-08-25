-- +goose Up
ALTER TABLE workspace_agents
ADD COLUMN icon TEXT NOT NULL DEFAULT 'bot'
CHECK (icon IN (
    'bot',
    'inbox',
    'search',
    'terminal',
    'pencil',
    'git-pull-request',
    'shield-check',
    'scroll-text',
    'target',
    'sparkles'
));

-- +goose Down
ALTER TABLE workspace_agents DROP COLUMN icon;
