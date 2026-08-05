-- +goose Up
CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    workspace_id uuid,
    workspace_name text,
    action text NOT NULL,
    outcome text NOT NULL,
    actor_account_id uuid,
    actor_kind text NOT NULL,
    actor_name text,
    actor_token_id uuid,
    actor_token_name text,
    actor_agent_id uuid,
    actor_agent_name text,
    auth_method text NOT NULL DEFAULT '',
    source_ip inet,
    user_agent text,
    resource_kind text NOT NULL,
    resource_id uuid,
    resource_name text,
    detail jsonb,
    CONSTRAINT audit_events_outcome_check CHECK (outcome IN ('succeeded', 'failed', 'denied')),
    CONSTRAINT audit_events_actor_kind_check
        CHECK (actor_kind IN ('user', 'token', 'agent', 'system'))
);

CREATE INDEX audit_events_workspace_idx ON audit_events (workspace_id, occurred_at DESC, id)
    WHERE workspace_id IS NOT NULL;

CREATE INDEX audit_events_instance_idx ON audit_events (occurred_at DESC, id);

CREATE INDEX audit_events_actor_idx ON audit_events (actor_account_id, occurred_at DESC)
    WHERE actor_account_id IS NOT NULL;

CREATE INDEX audit_events_action_idx ON audit_events (action, occurred_at DESC);

ALTER TABLE workspace_memberships
    ADD COLUMN reads_audit boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_memberships DROP COLUMN reads_audit;

DROP TABLE IF EXISTS audit_events;
