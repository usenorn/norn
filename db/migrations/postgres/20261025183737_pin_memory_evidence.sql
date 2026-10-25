-- +goose Up
ALTER TABLE workspace_memory_evidence
    DROP CONSTRAINT workspace_memory_evidence_issue_fkey,
    DROP CONSTRAINT workspace_memory_evidence_comment_fkey,
    DROP CONSTRAINT workspace_memory_evidence_doc_revision_fkey,
    ADD COLUMN issue_version integer,
    ADD COLUMN comment_version integer;

UPDATE workspace_memory_evidence e
SET issue_version = i.version
FROM workspace_issues i
WHERE e.kind = 'issue' AND i.id = e.issue_id;

UPDATE workspace_memory_evidence e
SET comment_version = c.version
FROM workspace_issue_comments c
WHERE e.kind = 'comment' AND c.id = e.comment_id;

ALTER TABLE workspace_memory_evidence
    ADD CONSTRAINT workspace_memory_evidence_pin_check CHECK (
        (kind = 'issue') = (issue_version IS NOT NULL) AND (kind = 'comment') = (comment_version IS NOT NULL)
    );

CREATE FUNCTION workspace_memory_evidence_state(
    cited_kind text,
    cited_issue_id uuid,
    cited_issue_version integer,
    cited_comment_id uuid,
    cited_comment_version integer,
    cited_doc_id uuid,
    cited_doc_revision_id uuid
) RETURNS text
    LANGUAGE sql
    STABLE
AS
$$
SELECT CASE cited_kind
           WHEN 'issue' THEN coalesce((
               SELECT CASE
                          WHEN i.status <> 'active'
                              OR greatest(coalesce((i.field_versions ->> 'title')::integer, 0),
                                          coalesce((i.field_versions ->> 'description')::integer, 0))
                                 > cited_issue_version THEN 'changed'
                          ELSE 'current'
                          END
               FROM workspace_issues i
               WHERE i.id = cited_issue_id), 'unavailable')
           WHEN 'comment' THEN coalesce((
               SELECT CASE
                          WHEN c.deleted_at IS NOT NULL THEN 'unavailable'
                          WHEN c.version > cited_comment_version THEN 'changed'
                          ELSE 'current'
                          END
               FROM workspace_issue_comments c
               WHERE c.id = cited_comment_id), 'unavailable')
           WHEN 'doc_revision' THEN coalesce((
               SELECT CASE
                          WHEN d.status = 'archived' OR EXISTS (
                              SELECT 1
                              FROM workspace_doc_revisions later
                              WHERE later.doc_id = d.id
                                AND later.version > pinned.version
                                AND later.action IN ('edited', 'restored', 'reshared')) THEN 'changed'
                          ELSE 'current'
                          END
               FROM workspace_doc_revisions pinned
               JOIN workspace_docs d ON d.id = pinned.doc_id
               WHERE pinned.id = cited_doc_revision_id AND pinned.doc_id = cited_doc_id), 'unavailable')
           ELSE 'current'
           END
$$;

-- +goose Down
DROP FUNCTION workspace_memory_evidence_state(text, uuid, integer, uuid, integer, uuid, uuid);

DELETE FROM workspace_memory_evidence e
WHERE (e.issue_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspace_issues i WHERE i.id = e.issue_id))
   OR (e.comment_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM workspace_issue_comments c WHERE c.id = e.comment_id))
   OR (e.doc_revision_id IS NOT NULL
       AND NOT EXISTS (SELECT 1 FROM workspace_doc_revisions r WHERE r.id = e.doc_revision_id));

ALTER TABLE workspace_memory_evidence
    DROP CONSTRAINT workspace_memory_evidence_pin_check,
    DROP COLUMN comment_version,
    DROP COLUMN issue_version,
    ADD CONSTRAINT workspace_memory_evidence_issue_fkey FOREIGN KEY (issue_id, workspace_id)
        REFERENCES workspace_issues (id, workspace_id) ON DELETE CASCADE,
    ADD CONSTRAINT workspace_memory_evidence_comment_fkey FOREIGN KEY (comment_id, workspace_id)
        REFERENCES workspace_issue_comments (id, workspace_id) ON DELETE CASCADE,
    ADD CONSTRAINT workspace_memory_evidence_doc_revision_fkey FOREIGN KEY (doc_revision_id, doc_id, workspace_id)
        REFERENCES workspace_doc_revisions (id, doc_id, workspace_id) ON DELETE CASCADE;
