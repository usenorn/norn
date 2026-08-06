-- +goose Up
CREATE TABLE workspace_webhooks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    owner_account_id uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name text NOT NULL,
    url text NOT NULL,
    events text[] NOT NULL,
    secret_sealed bytea NOT NULL,
    secret_hint text NOT NULL DEFAULT '',
    previous_secret_sealed bytea,
    previous_secret_expires_at timestamptz,
    enabled boolean NOT NULL DEFAULT true,
    disabled_at timestamptz,
    disabled_reason text NOT NULL DEFAULT '',
    failure_streak integer NOT NULL DEFAULT 0,
    last_delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_webhooks_events_check CHECK (cardinality(events) > 0),
    CONSTRAINT workspace_webhooks_disabled_check CHECK (enabled OR disabled_reason <> ''),
    CONSTRAINT workspace_webhooks_streak_check CHECK (failure_streak >= 0)
);

CREATE INDEX workspace_webhooks_workspace_idx
    ON workspace_webhooks (workspace_id, created_at DESC, id);

CREATE INDEX workspace_webhooks_subscribed_idx
    ON workspace_webhooks USING gin (events) WHERE enabled;

CREATE TABLE webhook_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    event text NOT NULL,
    subject_kind text NOT NULL DEFAULT '',
    subject_id uuid,
    team_id uuid,
    actor_account_id uuid,
    actor_kind text NOT NULL DEFAULT '',
    actor_name text NOT NULL DEFAULT '',
    body jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    fanned_out_at timestamptz
);

CREATE INDEX webhook_outbox_pending_idx
    ON webhook_outbox (occurred_at, id) WHERE fanned_out_at IS NULL;

CREATE INDEX webhook_outbox_sweep_idx ON webhook_outbox (occurred_at);

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id uuid NOT NULL REFERENCES workspace_webhooks (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    outbox_id uuid,
    replay_of uuid,
    event text NOT NULL,
    subject_kind text NOT NULL DEFAULT '',
    subject_id uuid,
    team_id uuid,
    body jsonb NOT NULL,
    state text NOT NULL DEFAULT 'pending',
    attempt integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    settled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_deliveries_state_check
        CHECK (state IN ('pending', 'succeeded', 'failed')),
    CONSTRAINT webhook_deliveries_attempt_check CHECK (attempt >= 0)
);

CREATE INDEX webhook_deliveries_webhook_idx
    ON webhook_deliveries (webhook_id, created_at DESC, id);

CREATE INDEX webhook_deliveries_due_idx
    ON webhook_deliveries (next_attempt_at, id) WHERE state = 'pending';

CREATE TABLE webhook_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    delivery_id uuid NOT NULL REFERENCES webhook_deliveries (id) ON DELETE CASCADE,
    attempt integer NOT NULL,
    request_url text NOT NULL DEFAULT '',
    resolved_address inet,
    outcome text NOT NULL,
    status_code integer,
    response_excerpt text NOT NULL DEFAULT '',
    error text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_attempts_outcome_check
        CHECK (outcome IN ('succeeded', 'rejected', 'refused', 'timed_out', 'failed'))
);

CREATE INDEX webhook_attempts_delivery_idx ON webhook_attempts (delivery_id, attempt);

-- +goose Down
DROP TABLE IF EXISTS webhook_attempts;

DROP TABLE IF EXISTS webhook_deliveries;

DROP TABLE IF EXISTS webhook_outbox;

DROP TABLE IF EXISTS workspace_webhooks;
