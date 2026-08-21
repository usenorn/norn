-- +goose Up
DROP INDEX IF EXISTS workspace_issue_delegations_claim_idx;

ALTER TABLE workspace_issue_delegations
    DROP CONSTRAINT IF EXISTS workspace_issue_delegations_claim_check,
    DROP COLUMN IF EXISTS claim_expires_at,
    DROP COLUMN IF EXISTS claim_claimed_at,
    DROP COLUMN IF EXISTS claim_token,
    DROP COLUMN IF EXISTS claim_runner;

-- +goose Down
ALTER TABLE workspace_issue_delegations
    ADD COLUMN claim_runner text,
    ADD COLUMN claim_token uuid,
    ADD COLUMN claim_claimed_at timestamptz,
    ADD COLUMN claim_expires_at timestamptz,
    ADD CONSTRAINT workspace_issue_delegations_claim_check
        CHECK (num_nulls(claim_runner, claim_token, claim_claimed_at, claim_expires_at) IN (0, 4));

CREATE INDEX workspace_issue_delegations_claim_idx
    ON workspace_issue_delegations (claim_expires_at) WHERE recalled_at IS NULL;
