-- +goose Up
ALTER TABLE workspace_storage_ledger
    ADD COLUMN max_bytes bigint,
    ADD CONSTRAINT workspace_storage_ledger_max_check CHECK (max_bytes IS NULL OR max_bytes >= 0);

CREATE TABLE workspace_import_files (
    object_key   text PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    size_bytes   bigint NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_import_files_size_check CHECK (size_bytes >= 0),
    CONSTRAINT workspace_import_files_key_check CHECK (object_key <> '')
);

CREATE INDEX workspace_import_files_workspace_idx
    ON workspace_import_files (workspace_id);

LOCK TABLE workspace_storage_ledger IN EXCLUSIVE MODE;

INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes, updated_at)
SELECT a.workspace_id, sum(a.size_bytes), now()
FROM workspace_issue_attachments a
GROUP BY a.workspace_id
ON CONFLICT (workspace_id) DO UPDATE
    SET stored_bytes = EXCLUDED.stored_bytes, updated_at = now()
    WHERE workspace_storage_ledger.stored_bytes <> EXCLUDED.stored_bytes;

UPDATE workspace_storage_ledger l
SET stored_bytes = 0, updated_at = now()
WHERE l.stored_bytes <> 0
  AND NOT EXISTS (
      SELECT 1 FROM workspace_issue_attachments a WHERE a.workspace_id = l.workspace_id
  );

-- +goose Down
DROP TABLE workspace_import_files;

ALTER TABLE workspace_storage_ledger
    DROP CONSTRAINT workspace_storage_ledger_max_check,
    DROP COLUMN max_bytes;
