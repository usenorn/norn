-- +goose Up
ALTER TABLE workspace_issue_delegations
    ADD COLUMN tool               text    NOT NULL DEFAULT '',
    ADD COLUMN model              text    NOT NULL DEFAULT '',
    ADD COLUMN runtime            text    NOT NULL DEFAULT '',
    ADD COLUMN base_ref           text    NOT NULL DEFAULT '',
    ADD COLUMN include_dirty      boolean NOT NULL DEFAULT false,
    ADD COLUMN permission_profile text    NOT NULL DEFAULT '',
    ADD CONSTRAINT workspace_issue_delegations_runtime_check
        CHECK (runtime IN ('', 'auto', 'process', 'docker')),
    ADD CONSTRAINT workspace_issue_delegations_base_ref_check
        CHECK (base_ref IN ('', 'origin/default', 'head')),
    ADD CONSTRAINT workspace_issue_delegations_profile_check
        CHECK (permission_profile IN ('', 'strict', 'standard', 'unrestricted'));

-- +goose Down
ALTER TABLE workspace_issue_delegations
    DROP CONSTRAINT workspace_issue_delegations_profile_check,
    DROP CONSTRAINT workspace_issue_delegations_base_ref_check,
    DROP CONSTRAINT workspace_issue_delegations_runtime_check,
    DROP COLUMN permission_profile,
    DROP COLUMN include_dirty,
    DROP COLUMN base_ref,
    DROP COLUMN runtime,
    DROP COLUMN model,
    DROP COLUMN tool;
