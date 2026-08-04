-- +goose Up
CREATE TABLE workspace_sso_identities (
    workspace_id uuid NOT NULL REFERENCES workspace_sso_connections (workspace_id) ON DELETE CASCADE,
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    subject      text NOT NULL,
    linked_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, account_id),
    CONSTRAINT workspace_sso_identities_subject_check CHECK (subject <> ''),
    CONSTRAINT workspace_sso_identities_subject_key UNIQUE (workspace_id, subject)
);

CREATE TABLE workspace_break_glass_codes (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    code_hash     bytea NOT NULL,
    issued_at     timestamptz NOT NULL DEFAULT now(),
    issued_by     uuid REFERENCES accounts (id) ON DELETE SET NULL,
    redeemed_at   timestamptz,
    redeemed_from text NOT NULL DEFAULT '',
    CONSTRAINT workspace_break_glass_codes_hash_key UNIQUE (code_hash),
    CONSTRAINT workspace_break_glass_codes_hash_length_check CHECK (octet_length(code_hash) = 32),
    CONSTRAINT workspace_break_glass_codes_redeemed_check
        CHECK (redeemed_at IS NOT NULL OR redeemed_from = '')
);

CREATE INDEX workspace_break_glass_codes_workspace_idx
    ON workspace_break_glass_codes (workspace_id, redeemed_at);

-- +goose Down
DROP TABLE workspace_break_glass_codes;

DROP TABLE workspace_sso_identities;
