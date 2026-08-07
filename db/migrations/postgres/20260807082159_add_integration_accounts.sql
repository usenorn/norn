-- +goose Up
ALTER TABLE accounts
    DROP CONSTRAINT accounts_kind_check,
    ADD CONSTRAINT accounts_kind_check CHECK (kind IN ('person', 'agent', 'integration'));

ALTER TABLE accounts
    DROP CONSTRAINT accounts_agent_credential_check,
    ADD CONSTRAINT accounts_machine_credential_check
        CHECK (kind = 'person' OR password_hash IS NULL);

ALTER TABLE accounts DROP CONSTRAINT accounts_active_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_active_identity_check
        CHECK (status <> 'active' OR (
            (kind <> 'person' OR email IS NOT NULL)
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NULL
            AND deleted_at IS NULL));

ALTER TABLE accounts DROP CONSTRAINT accounts_deactivated_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_deactivated_identity_check
        CHECK (status <> 'deactivated' OR (
            (kind <> 'person' OR email IS NOT NULL)
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NOT NULL
            AND deleted_at IS NULL));

ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed',
                        'code_linked', 'code_unlinked'));

-- +goose Down
ALTER TABLE workspace_activity
    DROP CONSTRAINT workspace_activity_kind_check,
    ADD CONSTRAINT workspace_activity_kind_check
        CHECK (kind IN ('created', 'state_changed', 'property_changed', 'team_moved',
                        'archived', 'unarchived', 'deleted', 'restored',
                        'child_added', 'child_removed',
                        'relation_added', 'relation_removed',
                        'triaged', 'commented', 'comment_deleted',
                        'member_added', 'member_removed',
                        'attachment_added', 'attachment_removed'));

ALTER TABLE accounts DROP CONSTRAINT accounts_deactivated_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_deactivated_identity_check
        CHECK (status <> 'deactivated' OR (
            email IS NOT NULL
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NOT NULL
            AND deleted_at IS NULL));

ALTER TABLE accounts DROP CONSTRAINT accounts_active_identity_check;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_active_identity_check
        CHECK (status <> 'active' OR (
            (kind = 'agent' OR email IS NOT NULL)
            AND display_name IS NOT NULL
            AND timezone IS NOT NULL
            AND deactivated_at IS NULL
            AND deleted_at IS NULL));

ALTER TABLE accounts
    DROP CONSTRAINT accounts_machine_credential_check,
    ADD CONSTRAINT accounts_agent_credential_check
        CHECK (kind <> 'agent' OR password_hash IS NULL);

ALTER TABLE accounts
    DROP CONSTRAINT accounts_kind_check,
    ADD CONSTRAINT accounts_kind_check CHECK (kind IN ('person', 'agent'));
