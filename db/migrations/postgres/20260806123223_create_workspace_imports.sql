-- +goose Up
CREATE TABLE workspace_import_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    source_kind text NOT NULL,
    source_label text NOT NULL DEFAULT '',
    requested_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    requested_actor_kind text NOT NULL DEFAULT 'user',
    requested_auth_method text NOT NULL DEFAULT '',
    reverted_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    reverted_actor_kind text NOT NULL DEFAULT '',
    reverted_auth_method text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    phase_error text NOT NULL DEFAULT '',
    preview_digest text NOT NULL DEFAULT '',
    acknowledge_triage boolean NOT NULL DEFAULT false,
    lease_token uuid,
    lease_expires_at timestamptz,
    attempt integer NOT NULL DEFAULT 0,
    staged integer NOT NULL DEFAULT 0,
    processed integer NOT NULL DEFAULT 0,
    staged_at timestamptz,
    mapped_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    reverted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_import_runs_source_check
        CHECK (source_kind <> '' AND length(source_kind) <= 64),
    CONSTRAINT workspace_import_runs_status_check
        CHECK (status IN ('draft', 'staging', 'staged', 'mapped', 'executing', 'imported', 'reverting', 'reverted', 'failed')),
    CONSTRAINT workspace_import_runs_requested_kind_check
        CHECK (requested_actor_kind IN ('user', 'token', 'agent')),
    CONSTRAINT workspace_import_runs_reverted_kind_check
        CHECK (reverted_actor_kind IN ('', 'user', 'token', 'agent')),
    CONSTRAINT workspace_import_runs_progress_check
        CHECK (staged >= 0 AND processed >= 0 AND attempt >= 0)
);

CREATE INDEX workspace_import_runs_workspace_idx
    ON workspace_import_runs (workspace_id, created_at DESC, id);

CREATE INDEX workspace_import_runs_lease_idx
    ON workspace_import_runs (lease_expires_at, id)
    WHERE status IN ('staging', 'executing', 'reverting');

CREATE TABLE workspace_import_cursors (
    run_id uuid NOT NULL REFERENCES workspace_import_runs (id) ON DELETE CASCADE,
    resource text NOT NULL,
    cursor text NOT NULL DEFAULT '',
    exhausted boolean NOT NULL DEFAULT false,
    fetched integer NOT NULL DEFAULT 0,
    retry_after timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, resource),
    CONSTRAINT workspace_import_cursors_fetched_check CHECK (fetched >= 0)
);

CREATE TABLE workspace_import_records (
    run_id uuid NOT NULL REFERENCES workspace_import_runs (id) ON DELETE CASCADE,
    resource text NOT NULL,
    external_id text NOT NULL,
    parent_external_id text NOT NULL DEFAULT '',
    payload jsonb NOT NULL,
    source_created_at timestamptz,
    source_updated_at timestamptz,
    state text NOT NULL DEFAULT 'staged',
    outcome_detail text NOT NULL DEFAULT '',
    created_id uuid,
    seq bigint GENERATED ALWAYS AS IDENTITY,
    PRIMARY KEY (run_id, resource, external_id),
    CONSTRAINT workspace_import_records_external_check CHECK (external_id <> ''),
    CONSTRAINT workspace_import_records_state_check
        CHECK (state IN ('staged', 'applied', 'skipped', 'failed'))
);

CREATE INDEX workspace_import_records_walk_idx
    ON workspace_import_records (run_id, resource, state, seq);

CREATE TABLE workspace_import_mappings (
    run_id uuid NOT NULL REFERENCES workspace_import_runs (id) ON DELETE CASCADE,
    kind text NOT NULL,
    source_key text NOT NULL,
    source_label text NOT NULL DEFAULT '',
    source_email text NOT NULL DEFAULT '',
    decision text NOT NULL,
    target_id uuid,
    target_value text NOT NULL DEFAULT '',
    suggested_target_id uuid,
    suggested_reason text NOT NULL DEFAULT '',
    decided_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    decided_at timestamptz,
    PRIMARY KEY (run_id, kind, source_key),
    CONSTRAINT workspace_import_mappings_kind_check
        CHECK (kind IN ('user', 'state', 'priority', 'label', 'project', 'team')),
    CONSTRAINT workspace_import_mappings_decision_check
        CHECK (decision IN ('map', 'create', 'unattributed', 'skip')),
    CONSTRAINT workspace_import_mappings_target_check
        CHECK (decision <> 'map' OR target_id IS NOT NULL OR target_value <> '')
);

CREATE TABLE workspace_import_ledger (
    run_id uuid NOT NULL REFERENCES workspace_import_runs (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL,
    resource text NOT NULL,
    created_id uuid NOT NULL,
    external_id text NOT NULL DEFAULT '',
    reference text NOT NULL DEFAULT '',
    version_at_create integer NOT NULL DEFAULT 0,
    created_at_recorded timestamptz NOT NULL DEFAULT now(),
    order_seq bigint GENERATED ALWAYS AS IDENTITY,
    reverted_at timestamptz,
    revert_outcome text NOT NULL DEFAULT '',
    PRIMARY KEY (run_id, resource, created_id),
    CONSTRAINT workspace_import_ledger_revert_check
        CHECK (revert_outcome IN ('', 'deleted', 'archived', 'retained', 'skipped_modified', 'skipped_in_use', 'failed'))
);

CREATE UNIQUE INDEX workspace_import_ledger_external_idx
    ON workspace_import_ledger (run_id, resource, external_id)
    WHERE external_id <> '';

CREATE INDEX workspace_import_ledger_walk_idx
    ON workspace_import_ledger (run_id, order_seq DESC)
    WHERE reverted_at IS NULL;

CREATE INDEX workspace_import_ledger_origin_idx
    ON workspace_import_ledger (workspace_id, resource, external_id)
    WHERE external_id <> '';

CREATE TABLE workspace_import_report_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES workspace_import_runs (id) ON DELETE CASCADE,
    phase text NOT NULL,
    resource text NOT NULL,
    subject text NOT NULL DEFAULT '',
    external_id text NOT NULL DEFAULT '',
    outcome text NOT NULL,
    detail jsonb,
    recorded_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_import_report_lines_phase_check
        CHECK (phase IN ('execute', 'revert')),
    CONSTRAINT workspace_import_report_lines_outcome_check
        CHECK (outcome IN ('created', 'skipped', 'unattributed', 'adjusted', 'dropped', 'failed', 'deleted', 'archived', 'retained', 'skipped_modified', 'skipped_in_use'))
);

CREATE INDEX workspace_import_report_lines_run_idx
    ON workspace_import_report_lines (run_id, phase, recorded_at, id);

-- +goose Down
DROP TABLE IF EXISTS workspace_import_report_lines;

DROP TABLE IF EXISTS workspace_import_ledger;

DROP TABLE IF EXISTS workspace_import_mappings;

DROP TABLE IF EXISTS workspace_import_records;

DROP TABLE IF EXISTS workspace_import_cursors;

DROP TABLE IF EXISTS workspace_import_runs;
