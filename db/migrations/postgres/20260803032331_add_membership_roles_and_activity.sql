-- +goose Up
ALTER TABLE workspace_memberships
    DROP CONSTRAINT workspace_memberships_role_check;

ALTER TABLE workspace_memberships
    ADD CONSTRAINT workspace_memberships_role_check
        CHECK (role IN ('admin', 'member', 'viewer'));

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_role_check;

ALTER TABLE workspace_invitations
    ADD CONSTRAINT workspace_invitations_role_check
        CHECK (role IN ('admin', 'member', 'viewer'));

ALTER TABLE workspace_memberships
    ADD COLUMN source           text NOT NULL DEFAULT 'manual',
    ADD COLUMN last_active_at   timestamptz,
    ADD COLUMN last_auth_method text;

ALTER TABLE workspace_memberships
    ADD CONSTRAINT workspace_memberships_source_check
        CHECK (source IN ('manual', 'directory')),
    ADD CONSTRAINT workspace_memberships_auth_method_check
        CHECK (last_auth_method IS NULL OR last_auth_method IN ('password', 'sso'));

-- +goose Down
ALTER TABLE workspace_memberships
    DROP CONSTRAINT workspace_memberships_auth_method_check,
    DROP CONSTRAINT workspace_memberships_source_check;

ALTER TABLE workspace_memberships
    DROP COLUMN last_auth_method,
    DROP COLUMN last_active_at,
    DROP COLUMN source;

UPDATE workspace_invitations SET role = 'member' WHERE role = 'viewer';

UPDATE workspace_memberships SET role = 'member' WHERE role = 'viewer';

ALTER TABLE workspace_invitations
    DROP CONSTRAINT workspace_invitations_role_check;

ALTER TABLE workspace_invitations
    ADD CONSTRAINT workspace_invitations_role_check
        CHECK (role IN ('admin', 'member'));

ALTER TABLE workspace_memberships
    DROP CONSTRAINT workspace_memberships_role_check;

ALTER TABLE workspace_memberships
    ADD CONSTRAINT workspace_memberships_role_check
        CHECK (role IN ('admin', 'member'));
