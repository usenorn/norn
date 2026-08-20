-- +goose Up
UPDATE workspace_memberships m
SET deactivated_at = a.disabled_at, updated_at = now()
FROM workspace_agents a
WHERE a.account_id = m.account_id
  AND a.workspace_id = m.workspace_id
  AND a.status = 'disabled'
  AND m.deactivated_at IS NULL;

-- +goose Down
UPDATE workspace_memberships m
SET deactivated_at = NULL, updated_at = now()
FROM workspace_agents a
WHERE a.account_id = m.account_id
  AND a.workspace_id = m.workspace_id
  AND a.status = 'disabled'
  AND m.deactivated_at = a.disabled_at;
