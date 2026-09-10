-- +goose Up
CREATE TABLE workspace_issue_criterion_evidence (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            uuid NOT NULL,
    issue_id                uuid NOT NULL,
    criterion_id            text NOT NULL,
    criterion_text          text NOT NULL DEFAULT '',
    kind                    text NOT NULL,
    label                   text NOT NULL DEFAULT '',
    url                     text NOT NULL DEFAULT '',
    attachment_id           uuid REFERENCES workspace_issue_attachments (id) ON DELETE SET NULL,
    description_revision_id uuid REFERENCES workspace_issue_description_revisions (id) ON DELETE SET NULL,
    recorded_by_account_id  uuid REFERENCES accounts (id) ON DELETE SET NULL,
    recorded_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_issue_criterion_evidence_issue_fkey
        FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    CONSTRAINT workspace_issue_criterion_evidence_kind_check
        CHECK (kind IN ('test', 'screenshot', 'pull_request', 'person')),
    CONSTRAINT workspace_issue_criterion_evidence_criterion_check
        CHECK (char_length(criterion_id) BETWEEN 1 AND 64),
    CONSTRAINT workspace_issue_criterion_evidence_label_check
        CHECK (char_length(label) <= 200),
    CONSTRAINT workspace_issue_criterion_evidence_url_check
        CHECK (char_length(url) <= 2048)
);

CREATE INDEX workspace_issue_criterion_evidence_issue_idx
    ON workspace_issue_criterion_evidence (issue_id, criterion_id, recorded_at DESC, id DESC);

-- +goose Down
DROP TABLE workspace_issue_criterion_evidence;
