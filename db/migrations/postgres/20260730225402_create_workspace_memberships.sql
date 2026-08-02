-- +goose Up
CREATE TABLE workspace_memberships (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    role         text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_memberships_role_check
        CHECK (role IN ('admin', 'member'))
);

CREATE UNIQUE INDEX workspace_memberships_workspace_account_key ON workspace_memberships (workspace_id, account_id);

CREATE INDEX workspace_memberships_account_id_idx ON workspace_memberships (account_id);

CREATE INDEX workspace_memberships_workspace_role_idx ON workspace_memberships (workspace_id, role);

-- +goose Down
DROP TABLE workspace_memberships;
