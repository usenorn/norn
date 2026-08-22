-- +goose Up
UPDATE workspace_invitations SET delivery = 'failed' WHERE delivery = 'link_only';

ALTER TABLE workspace_invitations DROP CONSTRAINT workspace_invitations_delivery_check;

ALTER TABLE workspace_invitations
    ADD CONSTRAINT workspace_invitations_delivery_check
        CHECK (delivery IN ('pending', 'sent', 'failed'));

-- +goose Down
ALTER TABLE workspace_invitations DROP CONSTRAINT workspace_invitations_delivery_check;

ALTER TABLE workspace_invitations
    ADD CONSTRAINT workspace_invitations_delivery_check
        CHECK (delivery IN ('pending', 'sent', 'failed', 'link_only'));
