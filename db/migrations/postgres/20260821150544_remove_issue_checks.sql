-- +goose Up
UPDATE workspace_team_agent_settings
SET hold_state_changes = 'always'
WHERE hold_state_changes = 'unless_proven';

ALTER TABLE workspace_team_agent_settings
    DROP CONSTRAINT workspace_team_agent_settings_state_changes_check,
    ADD CONSTRAINT workspace_team_agent_settings_state_changes_check
        CHECK (hold_state_changes IN ('never', 'always'));

DELETE FROM workspace_agent_proposals WHERE action = 'check_set';

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'issue_create'));

UPDATE workspace_agent_proposals
SET change = change - 'checkIds'
WHERE change ? 'checkIds';

DELETE FROM workspace_code_link_transitions
WHERE status = 'deferred' AND blocked_by IN ('checks_unproven', 'checks_unratified');

DELETE FROM workspace_activity
WHERE kind IN ('check_added', 'check_removed', 'check_approved', 'check_declined',
               'check_edited', 'check_waived', 'check_gap_declared', 'evidence_added',
               'checks_overridden', 'check_expired');

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
                        'code_linked', 'code_unlinked',
                        'delegated', 'recalled'));

DELETE FROM workspace_notification_events WHERE kind IN ('check_failed', 'gap_declared');

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership',
                        'approval_waiting'));

ALTER TABLE workspace_team_notification_settings
    DROP COLUMN email_checks,
    DROP COLUMN inbox_checks;

ALTER TABLE workspace_notification_settings
    DROP COLUMN email_checks,
    DROP COLUMN inbox_checks;

DROP TABLE workspace_check_evidence, workspace_issue_checks;

-- +goose Down
CREATE TABLE workspace_issue_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL,
    position integer NOT NULL DEFAULT 0,
    statement text NOT NULL,
    method text NOT NULL,
    proof text NOT NULL,
    time_limit_seconds integer,
    approval text NOT NULL DEFAULT 'pending',
    approved_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    approved_at timestamptz,
    resolution text NOT NULL DEFAULT 'none',
    resolution_reason text NOT NULL DEFAULT '',
    resolved_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    resolved_at timestamptz,
    gap_issue_id uuid REFERENCES workspace_issues (id) ON DELETE SET NULL,
    author_kind text NOT NULL DEFAULT 'user',
    created_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    added_after_delegation boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_checks_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_checks_statement_check CHECK (statement <> ''),
    CONSTRAINT workspace_issue_checks_proof_check CHECK (proof <> ''),
    CONSTRAINT workspace_issue_checks_method_check
        CHECK (method IN ('command', 'observation', 'manual', 'regression')),
    CONSTRAINT workspace_issue_checks_approval_check
        CHECK (approval IN ('pending', 'approved', 'declined')),
    CONSTRAINT workspace_issue_checks_approved_check
        CHECK ((approval = 'pending') = (approved_at IS NULL)),
    CONSTRAINT workspace_issue_checks_resolution_check
        CHECK (resolution IN ('none', 'waived', 'gap')),
    CONSTRAINT workspace_issue_checks_resolved_check
        CHECK ((resolution = 'none') = (resolved_at IS NULL)),
    CONSTRAINT workspace_issue_checks_reason_check
        CHECK (resolution = 'none' OR resolution_reason <> ''),
    CONSTRAINT workspace_issue_checks_gap_check
        CHECK (resolution = 'gap' OR gap_issue_id IS NULL),
    CONSTRAINT workspace_issue_checks_author_kind_check
        CHECK (author_kind IN ('user', 'token', 'agent', 'system')),
    CONSTRAINT workspace_issue_checks_time_limit_check
        CHECK (time_limit_seconds IS NULL OR time_limit_seconds > 0)
);

CREATE INDEX workspace_issue_checks_issue_idx
    ON workspace_issue_checks (issue_id, position, id);

CREATE INDEX workspace_issue_checks_gap_idx
    ON workspace_issue_checks (gap_issue_id) WHERE gap_issue_id IS NOT NULL;

CREATE TABLE workspace_check_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    check_id uuid NOT NULL REFERENCES workspace_issue_checks (id) ON DELETE CASCADE,
    issue_id uuid NOT NULL,
    verdict text NOT NULL,
    channel text NOT NULL,
    command text NOT NULL DEFAULT '',
    output text NOT NULL DEFAULT '',
    output_truncated boolean NOT NULL DEFAULT false,
    redactions integer NOT NULL DEFAULT 0,
    exit_code integer,
    observed_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    actor_kind text NOT NULL DEFAULT 'user',
    actor_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    code_link_id uuid REFERENCES workspace_code_links (id) ON DELETE SET NULL,
    commit_sha text NOT NULL DEFAULT '',
    scrubbed_by_account_id uuid REFERENCES accounts (id) ON DELETE SET NULL,
    scrubbed_at timestamptz,
    CONSTRAINT workspace_check_evidence_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_check_evidence_verdict_check
        CHECK (verdict IN ('passed', 'failed', 'absent_negative', 'inconclusive')),
    CONSTRAINT workspace_check_evidence_channel_check
        CHECK (channel IN ('command', 'http', 'log', 'screenshot', 'database', 'human')),
    CONSTRAINT workspace_check_evidence_actor_kind_check
        CHECK (actor_kind IN ('user', 'token', 'agent', 'system')),
    CONSTRAINT workspace_check_evidence_observed_check CHECK (observed_at <= received_at),
    CONSTRAINT workspace_check_evidence_output_check CHECK (octet_length(output) <= 65536),
    CONSTRAINT workspace_check_evidence_redactions_check CHECK (redactions >= 0),
    CONSTRAINT workspace_check_evidence_scrubbed_check
        CHECK ((scrubbed_at IS NULL) = (scrubbed_by_account_id IS NULL))
);

CREATE INDEX workspace_check_evidence_check_idx
    ON workspace_check_evidence (check_id, received_at DESC, id);

CREATE INDEX workspace_check_evidence_issue_idx
    ON workspace_check_evidence (issue_id, received_at DESC, id);

CREATE INDEX workspace_check_evidence_link_idx
    ON workspace_check_evidence (code_link_id) WHERE code_link_id IS NOT NULL;

ALTER TABLE workspace_issue_checks
    ADD COLUMN expiry_announced_for uuid
        REFERENCES workspace_check_evidence (id) ON DELETE SET NULL;

ALTER TABLE workspace_notification_settings
    ADD COLUMN inbox_checks boolean NOT NULL DEFAULT true,
    ADD COLUMN email_checks boolean NOT NULL DEFAULT false;

ALTER TABLE workspace_team_notification_settings
    ADD COLUMN inbox_checks boolean NOT NULL DEFAULT true,
    ADD COLUMN email_checks boolean NOT NULL DEFAULT false;

ALTER TABLE workspace_notification_events
    DROP CONSTRAINT workspace_notification_events_kind_check,
    ADD CONSTRAINT workspace_notification_events_kind_check
        CHECK (kind IN ('assigned', 'commented', 'state_changed', 'membership',
                        'check_failed', 'gap_declared', 'approval_waiting'));

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
                        'code_linked', 'code_unlinked',
                        'delegated', 'recalled',
                        'check_added', 'check_removed', 'check_approved', 'check_declined',
                        'check_edited', 'check_waived', 'check_gap_declared', 'evidence_added',
                        'checks_overridden', 'check_expired'));

ALTER TABLE workspace_agent_proposals
    DROP CONSTRAINT workspace_agent_proposals_action_check,
    ADD CONSTRAINT workspace_agent_proposals_action_check
        CHECK (action IN ('comment', 'state_change', 'issue_edit', 'issue_create', 'check_set'));

ALTER TABLE workspace_team_agent_settings
    DROP CONSTRAINT workspace_team_agent_settings_state_changes_check,
    ADD CONSTRAINT workspace_team_agent_settings_state_changes_check
        CHECK (hold_state_changes IN ('never', 'unless_proven', 'always'));
