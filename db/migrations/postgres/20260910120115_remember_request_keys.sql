-- +goose Up
CREATE TABLE workspace_request_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id   uuid REFERENCES accounts (id) ON DELETE CASCADE,
    scope        text NOT NULL,
    request_key  text NOT NULL,
    issue_id     uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_request_keys_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_request_keys_scope_check
        CHECK (scope IN ('issue')),
    CONSTRAINT workspace_request_keys_key_check
        CHECK (char_length(request_key) BETWEEN 1 AND 128)
);

CREATE UNIQUE INDEX workspace_request_keys_key
    ON workspace_request_keys (workspace_id, scope, coalesce(account_id, '00000000-0000-0000-0000-000000000000'::uuid), request_key);

CREATE INDEX workspace_request_keys_created_idx
    ON workspace_request_keys (created_at);

-- +goose Down
DROP TABLE workspace_request_keys;
