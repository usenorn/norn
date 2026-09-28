-- +goose Up
CREATE TABLE workspace_telegram_bots (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    agent_id                uuid NOT NULL UNIQUE REFERENCES workspace_agents (id) ON DELETE CASCADE,
    bot_user_id             bigint NOT NULL UNIQUE,
    bot_username            text NOT NULL,
    bot_name                text NOT NULL DEFAULT '',
    token_sealed            bytea NOT NULL,
    token_hint              text NOT NULL DEFAULT '',
    webhook_secret_hash     bytea NOT NULL,
    connected_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    connected_at            timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_telegram_bots_token_check CHECK (octet_length(token_sealed) > 0),
    CONSTRAINT workspace_telegram_bots_secret_check CHECK (octet_length(webhook_secret_hash) > 0),
    CONSTRAINT workspace_telegram_bots_username_check CHECK (bot_username <> '')
);

CREATE INDEX workspace_telegram_bots_workspace_idx ON workspace_telegram_bots (workspace_id);

CREATE TABLE workspace_telegram_link_codes (
    code_hash  bytea PRIMARY KEY,
    bot_id     uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    purpose    text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_telegram_link_codes_purpose_check CHECK (purpose IN ('private', 'group'))
);

CREATE INDEX workspace_telegram_link_codes_expiry_idx ON workspace_telegram_link_codes (expires_at);

CREATE TABLE workspace_telegram_accounts (
    bot_id            uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    account_id        uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    telegram_user_id  bigint NOT NULL,
    chat_id           bigint NOT NULL,
    telegram_username text NOT NULL DEFAULT '',
    linked_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (bot_id, account_id),
    CONSTRAINT workspace_telegram_accounts_user_key UNIQUE (bot_id, telegram_user_id)
);

CREATE TABLE workspace_telegram_groups (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id              uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    chat_id             bigint NOT NULL,
    title               text NOT NULL DEFAULT '',
    bound_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    bound_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_telegram_groups_chat_key UNIQUE (bot_id, chat_id)
);

CREATE TABLE workspace_telegram_updates (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id       uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    update_id    bigint NOT NULL,
    payload      jsonb,
    outcome      text NOT NULL DEFAULT '',
    received_at  timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    CONSTRAINT workspace_telegram_updates_update_key UNIQUE (bot_id, update_id),
    CONSTRAINT workspace_telegram_updates_outcome_check
        CHECK (outcome IN ('', 'applied', 'ignored', 'failed')),
    CONSTRAINT workspace_telegram_updates_processed_check
        CHECK ((processed_at IS NULL) = (outcome = ''))
);

CREATE INDEX workspace_telegram_updates_received_idx ON workspace_telegram_updates (received_at);

CREATE TABLE workspace_telegram_question_messages (
    bot_id      uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    chat_id     bigint NOT NULL,
    message_id  bigint NOT NULL,
    question_id uuid NOT NULL REFERENCES workspace_issue_questions (id) ON DELETE CASCADE,
    sent_at     timestamptz NOT NULL DEFAULT now(),
    settled_at  timestamptz,
    PRIMARY KEY (bot_id, chat_id, message_id),
    CONSTRAINT workspace_telegram_question_messages_question_key UNIQUE (question_id, bot_id, chat_id)
);

CREATE TABLE workspace_telegram_turns (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id     uuid NOT NULL REFERENCES workspace_telegram_bots (id) ON DELETE CASCADE,
    chat_id    bigint NOT NULL,
    role       text NOT NULL,
    text       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_telegram_turns_role_check CHECK (role IN ('user', 'assistant')),
    CONSTRAINT workspace_telegram_turns_text_check CHECK (text <> '')
);

CREATE INDEX workspace_telegram_turns_chat_idx ON workspace_telegram_turns (bot_id, chat_id, created_at DESC);

-- +goose Down
DROP TABLE workspace_telegram_turns;
DROP TABLE workspace_telegram_question_messages;
DROP TABLE workspace_telegram_updates;
DROP TABLE workspace_telegram_groups;
DROP TABLE workspace_telegram_accounts;
DROP TABLE workspace_telegram_link_codes;
DROP TABLE workspace_telegram_bots;
