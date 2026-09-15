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
       a.updated_at,
       a.settle_after,
       a.charged_until`

const attachmentJoins = `
FROM workspace_issue_attachments a
LEFT JOIN accounts acct ON acct.id = a.uploaded_by_account_id`

const createAttachmentQuery = `
WITH created AS (
    INSERT INTO workspace_issue_attachments
        (id, workspace_id, issue_id, comment_id, uploaded_by_account_id,
         object_key, file_name, content_type, size_bytes, status, reclaim_after,
         created_at, updated_at, settle_after, charged_until)
    VALUES ($1, $2, nullif($3, '')::uuid, nullif($4, '')::uuid, nullif($5, '')::uuid,
            $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    RETURNING *
)
SELECT` + attachmentColumns + `
FROM created a
LEFT JOIN accounts acct ON acct.id = a.uploaded_by_account_id`

const attachmentByIDQuery = `SELECT` + attachmentColumns + attachmentJoins + `
WHERE a.id = $1 AND a.workspace_id = $2`

const lockAttachmentQuery = attachmentByIDQuery + `
FOR UPDATE OF a`

const lockStoredAttachmentByKeyQuery = `SELECT` + attachmentColumns + attachmentJoins + `
WHERE a.object_key = $1 AND a.workspace_id = $2 AND a.status = 'stored'
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
SET status = 'discarded', size_bytes = 0, reclaim_after = $2, updated_at = $2
WHERE id = $1 AND status <> 'discarded' AND size_bytes = $3`

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
    WHERE coalesce(workspace_storage_ledger.max_bytes, $3::bigint) = 0
       OR workspace_storage_ledger.stored_bytes + $2::bigint
          <= coalesce(workspace_storage_ledger.max_bytes, $3::bigint)
RETURNING stored_bytes`

const releaseStorageQuery = `
INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes, updated_at)
VALUES ($1::uuid, 0, now())
ON CONFLICT (workspace_id) DO UPDATE
    SET stored_bytes = greatest(workspace_storage_ledger.stored_bytes - $2::bigint, 0), updated_at = now()`

const correctStorageQuery = `
INSERT INTO workspace_storage_ledger (workspace_id, stored_bytes, updated_at)
VALUES ($1::uuid, greatest($2::bigint, 0), now())
ON CONFLICT (workspace_id) DO UPDATE
    SET stored_bytes = greatest(workspace_storage_ledger.stored_bytes + $2::bigint, 0), updated_at = now()`

const ledgerQuery = `
SELECT coalesce(l.stored_bytes, 0), coalesce(l.max_bytes, $2::bigint), coalesce(l.updated_at, now())
FROM workspaces w
LEFT JOIN workspace_storage_ledger l ON l.workspace_id = w.id
WHERE w.id = $1`

const claimImportFileQuery = `
INSERT INTO workspace_import_files (object_key, workspace_id, size_bytes)
VALUES ($1, $2::uuid, 0)
ON CONFLICT (object_key) DO NOTHING`

const lockImportFileQuery = `
SELECT size_bytes, settle_after, charged_until
FROM workspace_import_files
WHERE object_key = $1 AND workspace_id = $2::uuid
FOR UPDATE`

const recordImportFileQuery = `
UPDATE workspace_import_files
SET size_bytes = $3, settle_after = $4, charged_until = $5, updated_at = now()
WHERE object_key = $1 AND workspace_id = $2::uuid`

const unsettledImportFilesQuery = `
SELECT workspace_id, object_key, size_bytes
FROM workspace_import_files
WHERE settle_after IS NOT NULL AND settle_after <= $1
ORDER BY settle_after, object_key
LIMIT $2`

const takeImportFileQuery = `
DELETE FROM workspace_import_files
WHERE object_key = $1 AND workspace_id = $2::uuid
RETURNING size_bytes, settle_after, charged_until`

const unsettledAttachmentsQuery = `
SELECT id, workspace_id
FROM workspace_issue_attachments
WHERE status = 'stored' AND settle_after IS NOT NULL AND settle_after <= $1
ORDER BY settle_after, id
LIMIT $2`

const measureAttachmentQuery = `
UPDATE workspace_issue_attachments
SET size_bytes = $2, settle_after = $3, charged_until = $4, updated_at = now()
WHERE id = $1 AND status = 'stored'`

const refundImportFileQuery = `
WITH refunded AS (
    DELETE FROM workspace_import_files
    WHERE object_key = $1 AND workspace_id = $2::uuid
    RETURNING workspace_id, size_bytes
)
UPDATE workspace_storage_ledger l
SET stored_bytes = greatest(l.stored_bytes - r.size_bytes, 0), updated_at = now()
FROM refunded r
WHERE l.workspace_id = r.workspace_id`

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
		settleAfter      sql.NullTime
		chargedUntil     sql.NullTime
	)

	if err := row.Scan(
		&id, &workspace, &issue, &comment, &uploader, &attachment.UploaderName,
		&attachment.ObjectKey, &attachment.FileName, &attachment.ContentType,
		&attachment.SizeBytes, &status, &reclaimAfter,
		&attachment.CreatedAt, &attachment.UpdatedAt, &settleAfter, &chargedUntil,
	); err != nil {
		return entity.Attachment{}, err
	}

	attachment.Status = entity.AttachmentStatus(status)

	if reclaimAfter.Valid {
		attachment.ReclaimAfter = &reclaimAfter.Time
	}

	if settleAfter.Valid {
		attachment.SettleAfter = &settleAfter.Time
	}

	if chargedUntil.Valid {
		attachment.ChargedUntil = &chargedUntil.Time
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

	createdAt, updatedAt := entity.OriginStamp(attachment.Origin, time.Now().UTC())

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
		createdAt,
		updatedAt,
		optionalTime(attachment.SettleAfter),
		optionalTime(attachment.ChargedUntil),
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

func (r *attachmentRepository) LockStoredByObjectKey(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
) (entity.Attachment, bool, error) {
	attachment, err := r.find(ctx, lockStoredAttachmentByKeyQuery, objectKey, workspaceID.String())
	if errors.Is(err, entity.ErrAttachmentNotFound) {
		return entity.Attachment{}, false, nil
	}

	if err != nil {
		return entity.Attachment{}, false, err
	}

	return attachment, true, nil
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
	releasedBytes int64,
	at time.Time,
) error {
	return r.touch(
		ctx, "discard attachment", discardAttachmentQuery,
		attachmentID.String(), at, releasedBytes,
	)
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

func (r *attachmentRepository) Correct(
	ctx context.Context,
	workspaceID uuid.UUID,
	deltaBytes int64,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, correctStorageQuery, workspaceID.String(), deltaBytes,
	); err != nil {
		return fmt.Errorf("correct workspace storage: %w", err)
	}

	return nil
}

func (r *attachmentRepository) Ledger(
	ctx context.Context,
	workspaceID uuid.UUID,
	defaultMaxBytes int64,
) (entity.WorkspaceStorage, error) {
	ledger := entity.WorkspaceStorage{WorkspaceID: workspaceID}

	err := r.db.Querier(ctx).QueryRowContext(ctx, ledgerQuery, workspaceID.String(), defaultMaxBytes).
		Scan(&ledger.StoredBytes, &ledger.MaxBytes, &ledger.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.WorkspaceStorage{}, entity.ErrWorkspaceNotFound
		}

		return entity.WorkspaceStorage{}, fmt.Errorf("read workspace storage: %w", err)
	}

	return ledger, nil
}

func (r *attachmentRepository) ClaimImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
) (entity.ImportFileCharge, error) {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(ctx, claimImportFileQuery, objectKey, workspaceID.String()); err != nil {
		return entity.ImportFileCharge{}, fmt.Errorf("claim import file: %w", err)
	}

	var (
		file                      = entity.ImportFileCharge{WorkspaceID: workspaceID, ObjectKey: objectKey}
		settleAfter, chargedUntil sql.NullTime
	)

	err := querier.QueryRowContext(ctx, lockImportFileQuery, objectKey, workspaceID.String()).
		Scan(&file.SizeBytes, &settleAfter, &chargedUntil)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ImportFileCharge{}, entity.ErrAttachmentNotFound
		}

		return entity.ImportFileCharge{}, fmt.Errorf("lock import file: %w", err)
	}

	if settleAfter.Valid {
		file.SettleAfter = &settleAfter.Time
	}

	if chargedUntil.Valid {
		file.ChargedUntil = &chargedUntil.Time
	}

	return file, nil
}

func (r *attachmentRepository) RecordImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
	sizeBytes int64,
	settleAfter, chargedUntil *time.Time,
) error {
	return r.touch(
		ctx, "record import file", recordImportFileQuery,
		objectKey, workspaceID.String(), sizeBytes, optionalTime(settleAfter), optionalTime(chargedUntil),
	)
}

func optionalTime(at *time.Time) any {
	if at == nil {
		return nil
	}

	return *at
}

func (r *attachmentRepository) ListUnsettledImportFiles(
	ctx context.Context,
	at time.Time,
	batch int,
) ([]entity.ImportFileCharge, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, unsettledImportFilesQuery, at, batch)
	if err != nil {
		return nil, fmt.Errorf("read unsettled import files: %w", err)
	}

	defer func() { _ = rows.Close() }()

	files := make([]entity.ImportFileCharge, 0)

	for rows.Next() {
		var (
			file      entity.ImportFileCharge
			workspace string
		)

		if err := rows.Scan(&workspace, &file.ObjectKey, &file.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan unsettled import file: %w", err)
		}

		if file.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return nil, fmt.Errorf("parse unsettled import file workspace id: %w", err)
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unsettled import files: %w", err)
	}

	return files, nil
}

func (r *attachmentRepository) TakeImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
) (entity.ImportFileCharge, bool, error) {
	var (
		file                      = entity.ImportFileCharge{WorkspaceID: workspaceID, ObjectKey: objectKey}
		settleAfter, chargedUntil sql.NullTime
	)

	err := r.db.Querier(ctx).QueryRowContext(ctx, takeImportFileQuery, objectKey, workspaceID.String()).
		Scan(&file.SizeBytes, &settleAfter, &chargedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.ImportFileCharge{}, false, nil
	}

	if err != nil {
		return entity.ImportFileCharge{}, false, fmt.Errorf("take import file: %w", err)
	}

	if settleAfter.Valid {
		file.SettleAfter = &settleAfter.Time
	}

	if chargedUntil.Valid {
		file.ChargedUntil = &chargedUntil.Time
	}

	return file, true, nil
}

func (r *attachmentRepository) ListUnsettledAttachments(
	ctx context.Context,
	at time.Time,
	batch int,
) ([]entity.Attachment, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, unsettledAttachmentsQuery, at, batch)
	if err != nil {
		return nil, fmt.Errorf("read unsettled attachments: %w", err)
	}

	defer func() { _ = rows.Close() }()

	attachments := make([]entity.Attachment, 0)

	for rows.Next() {
		var id, workspace string

		if err := rows.Scan(&id, &workspace); err != nil {
			return nil, fmt.Errorf("scan unsettled attachment: %w", err)
		}

		var attachment entity.Attachment

		if attachment.ID, err = uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("parse unsettled attachment id: %w", err)
		}

		if attachment.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return nil, fmt.Errorf("parse unsettled attachment workspace id: %w", err)
		}

		attachments = append(attachments, attachment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unsettled attachments: %w", err)
	}

	return attachments, nil
}

func (r *attachmentRepository) MeasureAttachment(
	ctx context.Context,
	attachmentID uuid.UUID,
	sizeBytes int64,
	settleAfter, chargedUntil *time.Time,
) error {
	return r.touch(
		ctx, "measure attachment", measureAttachmentQuery,
		attachmentID.String(), sizeBytes, optionalTime(settleAfter), optionalTime(chargedUntil),
	)
}

func (r *attachmentRepository) RefundImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, refundImportFileQuery, objectKey, workspaceID.String(),
	); err != nil {
		return fmt.Errorf("refund import file: %w", err)
	}

	return nil
}
