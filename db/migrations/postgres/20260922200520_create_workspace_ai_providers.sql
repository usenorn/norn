-- +goose Up
CREATE TABLE workspace_ai_providers (
    workspace_id          uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    provider              text NOT NULL,
    base_url              text NOT NULL DEFAULT '',
    allow_private_address boolean NOT NULL DEFAULT false,
    api_key_sealed        bytea NOT NULL,
    api_key_hint          text NOT NULL DEFAULT '',
    default_model         text NOT NULL DEFAULT '',
    verified_at           timestamptz,
    failed_at             timestamptz,
    failure               text NOT NULL DEFAULT '',
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_ai_providers_provider_check CHECK (provider IN ('openai')),
    CONSTRAINT workspace_ai_providers_key_check CHECK (octet_length(api_key_sealed) > 0),
    CONSTRAINT workspace_ai_providers_failure_check CHECK (
        failure IN (
            '', 'key_rejected', 'model_unavailable', 'quota_exceeded', 'rate_limited', 'unreachable',
            'destination_refused'
        )
    ),
    CONSTRAINT workspace_ai_providers_failed_check CHECK ((failed_at IS NULL) = (failure = ''))
);

-- +goose Down
DROP TABLE workspace_ai_providers;
