-- +goose Up
UPDATE api_tokens
SET revoked_at = now(),
    updated_at = now()
WHERE revoked_at IS NULL
  AND scopes = ARRAY['team:manage']
  AND account_id IN (SELECT account_id FROM workspace_agents);

UPDATE api_tokens
SET scopes = array_remove(scopes, 'team:manage'),
    updated_at = now()
WHERE cardinality(scopes) > 1
  AND 'team:manage' = ANY (scopes)
  AND account_id IN (SELECT account_id FROM workspace_agents);

-- +goose Down
-- A scope that was removed cannot be told apart from one never granted, so it is not restored.
SELECT 1;
