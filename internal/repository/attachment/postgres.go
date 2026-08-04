package attachment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type attachmentRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Attachment {
	return &attachmentRepository{db: db}
}

const attachmentColumns = `
       a.id,
       a.workspace_id,
       coalesce(a.issue_id::text, ''),
       coalesce(a.comment_id::text, ''),
       coalesce(a.uploaded_by_account_id::text, ''),
       coalesce(acct.display_name, ''),
       a.object_key,
       a.file_name,
       a.content_type,
       a.size_bytes,
       a.status,
       a.reclaim_after,
       a.created_at,
       a.updated_at`

const attachmentJoins = `
FROM workspace_issue_attachments a
LEFT JOIN accounts acct ON acct.id = a.uploaded_by_account_id`

const createAttachmentQuery = `
WITH created AS (
    INSERT INTO workspace_issue_attachments
        (id, workspace_id, issue_id, comment_id, uploaded_by_account_id,
         object_key, file_name, content_type, size_bytes, status, reclaim_after)
    VALUES ($1, $2, nullif($3, '')::uuid, nullif($4, '')::uuid, nullif($5, '')::uuid,
            $6, $7, $8, $9, $10, $11)
    RETURNING *
)
SELECT` + attachmentColumns + `
FROM created a
LEFT JOIN accounts acct ON acct.id = a.uploaded_by_account_id`

const attachmentByIDQuery = `SELECT` + attachmentColumns + attachmentJoins + `
WHERE a.id = $1 AND a.workspace_id = $2`

const lockAttachmentQuery = attachmentByIDQuery + `
FOR UPDATE OF a`

const attachmentsByIssueQuery = `SELECT` + attachmentColumns + attachmentJoins + `
WHERE a.issue_id = $1 AND a.status = 'stored'
ORDER BY a.created_at, a.id`

const settleAttachmentQuery = `
UPDATE workspace_issue_attachments
SET status = 'stored', size_bytes = $2, content_type = $3, reclaim_after = NULL, updated_at = $4
WHERE id = $1 AND status = 'pending'`

const discardAttachmentQuery = `
UPDATE workspace_issue_attachments
SET status = 'discarded', reclaim_after = $2, updated_at = $2
WHERE id = $1 AND status <> 'discarded'`

const claimForCommentQuery = `
UPDATE workspace_issue_attachments
SET comment_id = $3, updated_at = now()
WHERE workspace_id = $1
  AND issue_id = $2
  AND comment_id IS NULL
  AND id = ANY($4::uuid[])`

const markOrphansQuery = `
UPDATE workspace_issue_attachments
SET status = 'discarded', reclaim_after = $1, updated_at = $1
WHERE reclaim_after IS NULL AND issue_id IS NULL`

const reclaimableQuery = `SELECT` + attachmentColumns + attachmentJoins + `
WHERE a.reclaim_after IS NOT NULL AND a.reclaim_after <= $1
ORDER BY a.id
LIMIT $2`

const reclaimAttachmentQuery = `
WITH reclaimed AS (
    DELETE FROM workspace_issue_attachments
    WHERE id = $1
    RETURNING workspace_id, size_bytes
)
UPDATE workspace_storage_ledger l
SET stored_bytes = greatest(l.stored_bytes - r.size_bytes, 0), updated_at = now()
FROM reclaimed r
WHERE l.workspace_id = r.workspace_id`

const admitStorageQuery = `
INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes, updated_at)
SELECT $1::uuid, $2::bigint, now()
WHERE $3::bigint = 0 OR $2::bigint <= $3::bigint
ON CONFLICT (workspace_id) DO UPDATE
    SET stored_bytes = workspace_storage_ledger.stored_bytes + $2::bigint, updated_at = now()
    WHERE $3::bigint = 0 OR workspace_storage_ledger.stored_bytes + $2::bigint <= $3::bigint
RETURNING stored_bytes`

const releaseStorageQuery = `
INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes, updated_at)
VALUES ($1::uuid, 0, now())
ON CONFLICT (workspace_id) DO UPDATE
    SET stored_bytes = greatest(workspace_storage_ledger.stored_bytes - $2::bigint, 0), updated_at = now()`

const ledgerQuery = `
SELECT coalesce(l.stored_bytes, 0), coalesce(l.updated_at, now())
FROM workspaces w
LEFT JOIN workspace_storage_ledger l ON l.workspace_id = w.id
WHERE w.id = $1`

type scanner interface {
	Scan(dest ...any) error
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func identifiers(ids []uuid.UUID) []string {
	raw := make([]string, 0, len(ids))
	for _, id := range ids {
		raw = append(raw, id.String())
	}

	return raw
}

func scanAttachment(row scanner) (entity.Attachment, error) {
	var (
		attachment       entity.Attachment
		id, workspace    string
		issue, comment   string
		uploader, status string
		reclaimAfter     sql.NullTime
	)

	if err := row.Scan(
		&id, &workspace, &issue, &comment, &uploader, &attachment.UploaderName,
		&attachment.ObjectKey, &attachment.FileName, &attachment.ContentType,
		&attachment.SizeBytes, &status, &reclaimAfter,
		&attachment.CreatedAt, &attachment.UpdatedAt,
	); err != nil {
		return entity.Attachment{}, err
	}

	attachment.Status = entity.AttachmentStatus(status)

	if reclaimAfter.Valid {
		attachment.ReclaimAfter = &reclaimAfter.Time
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Attachment{}, fmt.Errorf("parse attachment id: %w", err)
	}

	attachment.ID = parsed

	if attachment.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Attachment{}, fmt.Errorf("parse attachment workspace id: %w", err)
	}

	for raw, target := range map[string]*uuid.UUID{
		issue:    &attachment.IssueID,
		comment:  &attachment.CommentID,
		uploader: &attachment.UploaderID,
	} {
		if raw == "" {
			continue
		}

		if *target, err = uuid.Parse(raw); err != nil {
			return entity.Attachment{}, fmt.Errorf("parse attachment reference: %w", err)
		}
	}

	return attachment, nil
}

func (r *attachmentRepository) Create(
	ctx context.Context,
	attachment entity.Attachment,
) (entity.Attachment, error) {
	if attachment.ID == uuid.Nil {
		attachment.ID = uuid.New()
	}

	var reclaimAfter any
	if attachment.ReclaimAfter != nil {
		reclaimAfter = *attachment.ReclaimAfter
	}

	created, err := scanAttachment(r.db.Querier(ctx).QueryRowContext(
		ctx, createAttachmentQuery,
		attachment.ID.String(),
		attachment.WorkspaceID.String(),
		text(attachment.IssueID),
		text(attachment.CommentID),
		text(attachment.UploaderID),
		attachment.ObjectKey,
		attachment.FileName,
		attachment.ContentType,
		attachment.SizeBytes,
		string(attachment.Status),
		reclaimAfter,
	))
	if err != nil {
		return entity.Attachment{}, fmt.Errorf("create attachment: %w", err)
	}

	return created, nil
}

func (r *attachmentRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.Attachment, error) {
	attachment, err := scanAttachment(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Attachment{}, entity.ErrAttachmentNotFound
		}

		return entity.Attachment{}, fmt.Errorf("find attachment: %w", err)
	}

	return attachment, nil
}

func (r *attachmentRepository) GetByID(
	ctx context.Context,
	workspaceID, attachmentID uuid.UUID,
) (entity.Attachment, error) {
	return r.find(ctx, attachmentByIDQuery, attachmentID.String(), workspaceID.String())
}

func (r *attachmentRepository) LockByID(
	ctx context.Context,
	workspaceID, attachmentID uuid.UUID,
) (entity.Attachment, error) {
	return r.find(ctx, lockAttachmentQuery, attachmentID.String(), workspaceID.String())
}

func (r *attachmentRepository) query(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.Attachment, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read attachments: %w", err)
	}

	defer func() { _ = rows.Close() }()

	attachments := make([]entity.Attachment, 0)

	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}

		attachments = append(attachments, attachment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}

	return attachments, nil
}

func (r *attachmentRepository) ListByIssue(
	ctx context.Context,
	issueID uuid.UUID,
) ([]entity.Attachment, error) {
	return r.query(ctx, attachmentsByIssueQuery, issueID.String())
}

func (r *attachmentRepository) ListReclaimable(
	ctx context.Context,
	at time.Time,
	batch int,
) ([]entity.Attachment, error) {
	return r.query(ctx, reclaimableQuery, at, batch)
}

func (r *attachmentRepository) Settle(
	ctx context.Context,
	attachmentID uuid.UUID,
	sizeBytes int64,
	contentType string,
	at time.Time,
) error {
	return r.touch(
		ctx, "settle attachment", settleAttachmentQuery,
		attachmentID.String(), sizeBytes, contentType, at,
	)
}

func (r *attachmentRepository) Discard(
	ctx context.Context,
	attachmentID uuid.UUID,
	at time.Time,
) error {
	return r.touch(ctx, "discard attachment", discardAttachmentQuery, attachmentID.String(), at)
}

func (r *attachmentRepository) touch(
	ctx context.Context,
	action, query string,
	args ...any,
) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	if affected == 0 {
		return entity.ErrAttachmentNotFound
	}

	return nil
}

func (r *attachmentRepository) ClaimForComment(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	attachmentIDs []uuid.UUID,
) error {
	if len(attachmentIDs) == 0 {
		return nil
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, claimForCommentQuery,
		workspaceID.String(), issueID.String(), commentID.String(), identifiers(attachmentIDs),
	); err != nil {
		return fmt.Errorf("claim attachments for comment: %w", err)
	}

	return nil
}

func (r *attachmentRepository) MarkOrphans(ctx context.Context, at time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(ctx, markOrphansQuery, at); err != nil {
		return fmt.Errorf("mark orphaned attachments: %w", err)
	}

	return nil
}

func (r *attachmentRepository) Reclaim(ctx context.Context, attachmentID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, reclaimAttachmentQuery, attachmentID.String(),
	); err != nil {
		return fmt.Errorf("reclaim attachment: %w", err)
	}

	return nil
}

func (r *attachmentRepository) Admit(
	ctx context.Context,
	workspaceID uuid.UUID,
	sizeBytes, maxBytes int64,
) (int64, error) {
	var stored int64

	err := r.db.Querier(ctx).QueryRowContext(
		ctx, admitStorageQuery, workspaceID.String(), sizeBytes, maxBytes,
	).Scan(&stored)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, entity.ErrStorageExhausted
		}

		return 0, fmt.Errorf("admit workspace storage: %w", err)
	}

	return stored, nil
}

func (r *attachmentRepository) Release(
	ctx context.Context,
	workspaceID uuid.UUID,
	sizeBytes int64,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, releaseStorageQuery, workspaceID.String(), sizeBytes,
	); err != nil {
		return fmt.Errorf("release workspace storage: %w", err)
	}

	return nil
}

func (r *attachmentRepository) Ledger(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceStorage, error) {
	ledger := entity.WorkspaceStorage{WorkspaceID: workspaceID}

	err := r.db.Querier(ctx).QueryRowContext(ctx, ledgerQuery, workspaceID.String()).
		Scan(&ledger.StoredBytes, &ledger.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.WorkspaceStorage{}, entity.ErrWorkspaceNotFound
		}

		return entity.WorkspaceStorage{}, fmt.Errorf("read workspace storage: %w", err)
	}

	return ledger, nil
}
