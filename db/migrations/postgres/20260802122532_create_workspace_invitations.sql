-- +goose Up
CREATE TABLE workspace_invitations (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id           uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    email                  text NOT NULL,
    role                   text NOT NULL,
    status                 text NOT NULL,
    delivery               text NOT NULL,
    token_hash             bytea NOT NULL,
    invited_by_account_id  uuid REFERENCES accounts (id) ON DELETE SET NULL,
    invited_at             timestamptz NOT NULL DEFAULT now(),
    expires_at             timestamptz NOT NULL,
    accepted_at            timestamptz,
    accepted_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    revoked_at             timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_invitations_role_check
        CHECK (role IN ('admin', 'member')),
    CONSTRAINT workspace_invitations_status_check
        CHECK (status IN ('pending', 'accepted', 'revoked')),
    CONSTRAINT workspace_invitations_delivery_check
        CHECK (delivery IN ('pending', 'sent', 'failed', 'link_only')),
    CONSTRAINT workspace_invitations_accepted_check
        CHECK (status <> 'accepted' OR accepted_at IS NOT NULL),
    CONSTRAINT workspace_invitations_revoked_check
        CHECK (status <> 'revoked' OR revoked_at IS NOT NULL)
);

CREATE UNIQUE INDEX workspace_invitations_token_key ON workspace_invitations (token_hash);

CREATE UNIQUE INDEX workspace_invitations_pending_key
    ON workspace_invitations (workspace_id, lower(email)) WHERE status = 'pending';

CREATE INDEX workspace_invitations_workspace_status_idx
    ON workspace_invitations (workspace_id, status);

-- +goose Down
DROP TABLE workspace_invitations;
