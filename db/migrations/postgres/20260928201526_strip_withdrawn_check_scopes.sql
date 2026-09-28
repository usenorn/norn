-- +goose Up
UPDATE api_tokens
SET revoked_at = now(),
    updated_at = now()
WHERE revoked_at IS NULL
  AND scopes <@ ARRAY['check:read', 'check:manage']::text[];

UPDATE api_tokens
SET scopes     = array_remove(array_remove(scopes, 'check:read'), 'check:manage'),
    updated_at = now()
WHERE scopes && ARRAY['check:read', 'check:manage']::text[]
  AND NOT scopes <@ ARRAY['check:read', 'check:manage']::text[];

-- +goose Down
SELECT 1;
