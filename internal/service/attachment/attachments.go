package attachment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const abandonGrace = time.Hour

type attachmentsService struct {
	attachments repository.Attachment
	issues      repository.Issue
	blobs       repository.Blob
	jobs        repository.JobProducer
	authorizer  service.Authorizer
	transactor  repository.Transactor
	cfg         config.Attachments
}

func New(
	attachments repository.Attachment,
	issues repository.Issue,
	blobs repository.Blob,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	cfg config.Attachments,
) service.Attachments {
	return &attachmentsService{
		attachments: attachments,
		issues:      issues,
		blobs:       blobs,
		jobs:        jobs,
		authorizer:  authorizer,
		transactor:  transactor,
		cfg:         cfg,
	}
}

func (s *attachmentsService) onVisibleIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	action entity.Action,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *attachmentsService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Attachment, error) {
	if _, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.attachments.ListByIssue(ctx, issueID)
}

func (s *attachmentsService) Reserve(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ReserveAttachmentInput,
) (service.AttachmentReservation, error) {
	decision, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return service.AttachmentReservation{}, err
	}

	name := entity.AttachmentFileName(input.FileName)

	if err := entity.NewValidationError(
		entity.ValidateAttachmentName("fileName", name),
	); err != nil {
		return service.AttachmentReservation{}, err
	}

	if input.SizeBytes <= 0 || input.SizeBytes > s.cfg.MaxFileBytes {
		return service.AttachmentReservation{}, entity.AttachmentTooLargeError{
			SizeBytes: input.SizeBytes,
			MaxBytes:  s.cfg.MaxFileBytes,
		}
	}

	attachmentID := uuid.New()
	reclaimAfter := time.Now().UTC().Add(s.cfg.UploadTTL + abandonGrace)

	reserved := entity.Attachment{
		ID:           attachmentID,
		WorkspaceID:  workspaceID,
		IssueID:      issueID,
		UploaderID:   decision.Actor.AccountID,
		ObjectKey:    entity.AttachmentKey(workspaceID, attachmentID),
		FileName:     name,
		ContentType:  entity.AttachmentGenericType,
		SizeBytes:    input.SizeBytes,
		Status:       entity.AttachmentStatusPending,
		ReclaimAfter: &reclaimAfter,
	}

	var created entity.Attachment

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if _, err := s.attachments.Admit(
			ctx, workspaceID, input.SizeBytes, s.cfg.MaxWorkspaceBytes,
		); err != nil {
			return s.exhausted(ctx, workspaceID, input.SizeBytes, err)
		}

		created, err = s.attachments.Create(ctx, reserved)

		return err
	}); err != nil {
		return service.AttachmentReservation{}, err
	}

	ticket, err := s.blobs.PresignPut(ctx, created.ObjectKey, s.cfg.UploadTTL)
	if err != nil {
		return service.AttachmentReservation{}, err
	}

	return service.AttachmentReservation{Attachment: created, Transfer: ticket}, nil
}

func (s *attachmentsService) exhausted(
	ctx context.Context,
	workspaceID uuid.UUID,
	sizeBytes int64,
	err error,
) error {
	if !errors.Is(err, entity.ErrStorageExhausted) {
		return err
	}

	ledger, readErr := s.attachments.Ledger(ctx, workspaceID)
	if readErr != nil {
		return err
	}

	return entity.StorageExhaustedError{
		SizeBytes:   sizeBytes,
		StoredBytes: ledger.StoredBytes,
		MaxBytes:    s.cfg.MaxWorkspaceBytes,
	}
}

func (s *attachmentsService) Finalize(
	ctx context.Context,
	workspaceID, issueID, attachmentID uuid.UUID,
) (entity.Attachment, error) {
	if _, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage); err != nil {
		return entity.Attachment{}, err
	}

	reserved, err := s.attachments.GetByID(ctx, workspaceID, attachmentID)
	if err != nil {
		return entity.Attachment{}, err
	}

	if reserved.IssueID != issueID {
		return entity.Attachment{}, entity.ErrAttachmentNotFound
	}

	if reserved.Status != entity.AttachmentStatusPending {
		return entity.Attachment{}, entity.ErrAttachmentNotPending
	}

	object, err := s.blobs.Stat(ctx, reserved.ObjectKey)
	if err != nil {
		if errors.Is(err, entity.ErrBlobNotFound) {
			return entity.Attachment{}, entity.ErrAttachmentMissing
		}

		return entity.Attachment{}, err
	}

	sniffed, err := s.blobs.Sniff(ctx, reserved.ObjectKey)
	if err != nil {
		return entity.Attachment{}, err
	}

	served := entity.AttachmentServedType(sniffed)

	if object.Size > s.cfg.MaxFileBytes {
		return entity.Attachment{}, s.refuse(ctx, reserved, entity.AttachmentTooLargeError{
			SizeBytes: object.Size,
			MaxBytes:  s.cfg.MaxFileBytes,
		})
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if delta := object.Size - reserved.SizeBytes; delta > 0 {
			if _, err := s.attachments.Admit(
				ctx, workspaceID, delta, s.cfg.MaxWorkspaceBytes,
			); err != nil {
				return s.exhausted(ctx, workspaceID, delta, err)
			}
		} else if delta < 0 {
			if err := s.attachments.Release(ctx, workspaceID, -delta); err != nil {
				return err
			}
		}

		return s.attachments.Settle(ctx, attachmentID, object.Size, served, time.Now().UTC())
	}); err != nil {
		if errors.Is(err, entity.ErrStorageExhausted) {
			return entity.Attachment{}, s.refuse(ctx, reserved, err)
		}

		return entity.Attachment{}, err
	}

	return s.attachments.GetByID(ctx, workspaceID, attachmentID)
}

func (s *attachmentsService) refuse(
	ctx context.Context,
	reserved entity.Attachment,
	refusal error,
) error {
	now := time.Now().UTC()

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.attachments.Release(ctx, reserved.WorkspaceID, reserved.SizeBytes); err != nil {
			return err
		}

		return s.attachments.Discard(ctx, reserved.ID, now)
	}); err != nil {
		return err
	}

	s.kick(ctx)

	return refusal
}

func (s *attachmentsService) Remove(
	ctx context.Context,
	workspaceID, issueID, attachmentID uuid.UUID,
) error {
	if _, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage); err != nil {
		return err
	}

	attachment, err := s.attachments.GetByID(ctx, workspaceID, attachmentID)
	if err != nil {
		return err
	}

	if attachment.IssueID != issueID {
		return entity.ErrAttachmentNotFound
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.attachments.Release(ctx, workspaceID, attachment.SizeBytes); err != nil {
			return err
		}

		return s.attachments.Discard(ctx, attachmentID, time.Now().UTC())
	}); err != nil {
		return err
	}

	s.kick(ctx)

	return nil
}

func (s *attachmentsService) Content(
	ctx context.Context,
	workspaceID, attachmentID uuid.UUID,
) (string, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return "", err
	}

	attachment, err := s.attachments.GetByID(ctx, workspaceID, attachmentID)
	if err != nil {
		return "", err
	}

	if !attachment.Stored() || attachment.Orphaned() {
		return "", entity.ErrAttachmentNotFound
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, attachment.IssueID, decision.Scope); err != nil {
		return "", err
	}

	return s.blobs.PresignGet(ctx, attachment.ObjectKey, entity.ServeSpec{
		ContentType: attachment.ContentType,
		Disposition: attachment.Disposition(),
		FileName:    attachment.FileName,
	}, s.cfg.LinkTTL)
}

func (s *attachmentsService) Ledger(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceStorage, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: workspaceID,
	}); err != nil {
		return entity.WorkspaceStorage{}, err
	}

	ledger, err := s.attachments.Ledger(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceStorage{}, err
	}

	ledger.MaxBytes = s.cfg.MaxWorkspaceBytes

	return ledger, nil
}

func (s *attachmentsService) Reclaim(ctx context.Context) error {
	now := time.Now().UTC()

	if err := s.attachments.MarkOrphans(ctx, now); err != nil {
		return err
	}

	reclaimable, err := s.attachments.ListReclaimable(ctx, now, s.cfg.ReclaimBatch)
	if err != nil {
		return err
	}

	failures := 0

	for _, attachment := range reclaimable {
		if err := s.release(ctx, attachment); err != nil {
			failures++

			logging.From(ctx).WarnContext(
				ctx, "reclaiming a stored file failed",
				"attachment_id", attachment.ID.String(),
				"object_key", attachment.ObjectKey,
				"error", err.Error(),
			)
		}
	}

	if failures > 0 {
		return fmt.Errorf("reclaim %d of %d stored files", failures, len(reclaimable))
	}

	return nil
}

func (s *attachmentsService) release(ctx context.Context, attachment entity.Attachment) error {
	if err := s.blobs.Delete(ctx, attachment.ObjectKey); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		return s.attachments.Reclaim(ctx, attachment.ID)
	})
}

func (s *attachmentsService) kick(ctx context.Context) {
	if err := s.jobs.EnqueueAttachmentReclaim(ctx); err != nil {
		logging.From(ctx).WarnContext(ctx, "scheduling a storage sweep failed", "error", err.Error())
	}
}
