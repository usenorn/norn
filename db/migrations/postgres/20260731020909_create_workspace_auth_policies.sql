-- +goose Up
CREATE TABLE workspace_auth_policies (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    enforcement  text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_auth_policies_enforcement_check
        CHECK (enforcement IN ('any', 'sso'))
);

-- +goose Down
DROP TABLE workspace_auth_policies;
