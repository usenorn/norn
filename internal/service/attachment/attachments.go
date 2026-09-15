package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
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
	activity    repository.Activity
	issues      repository.Issue
	blobs       repository.Blob
	jobs        repository.JobProducer
	authorizer  service.Authorizer
	transactor  repository.Transactor
	cfg         config.Attachments
}

func New(
	attachments repository.Attachment,
	activity repository.Activity,
	issues repository.Issue,
	blobs repository.Blob,
	jobs repository.JobProducer,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	cfg config.Attachments,
) service.Attachments {
	return &attachmentsService{
		attachments: attachments,
		activity:    activity,
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
) (entity.Decision, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Decision{}, err
	}

	if err := s.issues.VisibleExists(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return entity.Decision{}, err
	}

	return decision, nil
}

func (s *attachmentsService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.Attachment, error) {
	if _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.attachments.ListByIssue(ctx, issueID)
}

func (s *attachmentsService) Reserve(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ReserveAttachmentInput,
) (service.AttachmentReservation, error) {
	created, err := s.reserve(ctx, workspaceID, issueID, input)
	if err != nil {
		return service.AttachmentReservation{}, err
	}

	ticket, err := s.blobs.PresignPut(ctx, created.ObjectKey, created.SizeBytes, s.cfg.UploadTTL)
	if err != nil {
		return service.AttachmentReservation{}, err
	}

	return service.AttachmentReservation{Attachment: created, Transfer: ticket}, nil
}

// Receive stores a file nobody uploaded. Mail arrives at a worker with the bytes already in
// hand, so there is no browser to hand a ticket to and no second request to settle the row:
// the same reservation is made, the object is written here, and the same finish runs.
func (s *attachmentsService) Receive(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ReceiveAttachmentInput,
) (entity.Attachment, error) {
	size := int64(len(input.Content))

	reserved, err := s.reserve(ctx, workspaceID, issueID, service.ReserveAttachmentInput{
		FileName:    input.FileName,
		ContentType: input.ContentType,
		SizeBytes:   size,
	})
	if err != nil {
		return entity.Attachment{}, err
	}

	if err := s.blobs.Put(
		ctx,
		reserved.ObjectKey,
		entity.AttachmentServedType(input.ContentType),
		bytes.NewReader(input.Content),
		size,
	); err != nil {
		return entity.Attachment{}, err
	}

	return s.Finalize(ctx, workspaceID, issueID, reserved.ID)
}

func (s *attachmentsService) reserve(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ReserveAttachmentInput,
) (entity.Attachment, error) {
	decision, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return entity.Attachment{}, err
	}

	name := entity.AttachmentFileName(input.FileName)

	if err := entity.NewValidationError(
		entity.ValidateAttachmentName("fileName", name),
	); err != nil {
		return entity.Attachment{}, err
	}

	if input.SizeBytes <= 0 || input.SizeBytes > s.cfg.MaxFileBytes {
		return entity.Attachment{}, entity.AttachmentTooLargeError{
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
		return entity.Attachment{}, err
	}

	return created, nil
}

// Reserve mints a presigned PUT for a browser and settles the row only once the bytes have
// arrived. A worker that already holds the object has neither a browser to hand a ticket to
// nor a second phase to drive, so adoption is its own entry point rather than a flag on the
// reservation. The origin is tested for attribution rather than presence: one decoded from a
// request body is non-nil and inert, so a nil check would open this to any caller.
func (s *attachmentsService) Adopt(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.AdoptAttachmentInput,
) (entity.Attachment, error) {
	if !entity.OriginAttributed(input.Origin) {
		return entity.Attachment{}, entity.ErrAttachmentAdoptNeedsOrigin
	}

	decision, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return entity.Attachment{}, err
	}

	name := entity.AttachmentFileName(input.FileName)

	if err := entity.NewValidationError(
		entity.ValidateAttachmentName("fileName", name),
	); err != nil {
		return entity.Attachment{}, err
	}

	// The key must name an object this workspace's own import wrote. Adopting takes ownership
	// of whatever it is handed, and a revert later deletes it: a key pointing into another
	// workspace's stored files would have this import take one and the undo destroy it.
	if !importKeyOf(workspaceID, input.ObjectKey) {
		return entity.Attachment{}, entity.NewValidationError(entity.FieldError{
			Field: "objectKey",
			Code:  entity.ValidationCodeMalformed,
		})
	}

	// The row is written stored, on its issue and with no reclaim deadline, because the sweep
	// collects on reclaim_after IS NULL AND issue_id IS NULL: a row that passed through pending
	// would be the file of a live import, deleted out from under it within the sweep interval.
	adopted := entity.Attachment{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		IssueID:     issueID,
		CommentID:   input.CommentID,
		UploaderID:  entity.OriginAuthor(input.Origin, decision.Actor.AccountID),
		ObjectKey:   input.ObjectKey,
		FileName:    name,
		ContentType: entity.AttachmentServedType(input.ContentType),
		SizeBytes:   input.SizeBytes,
		Status:      entity.AttachmentStatusStored,
		Origin:      input.Origin,
	}

	var created entity.Attachment

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		file, charged, err := s.attachments.TakeImportFile(ctx, workspaceID, input.ObjectKey)
		if err != nil {
			return err
		}

		if charged {
			adopted.SizeBytes = file.SizeBytes
			adopted.SettleAfter = file.SettleAfter
			adopted.ChargedUntil = file.ChargedUntil
		} else if _, err := s.attachments.Admit(
			ctx, workspaceID, input.SizeBytes, s.cfg.MaxWorkspaceBytes,
		); err != nil {
			return s.exhausted(ctx, workspaceID, input.SizeBytes, err)
		}

		created, err = s.attachments.Create(ctx, adopted)

		return err
	}); err != nil {
		return entity.Attachment{}, err
	}

	return created, nil
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

	ledger, readErr := s.attachments.Ledger(ctx, workspaceID, s.cfg.MaxWorkspaceBytes)
	if readErr != nil {
		return err
	}

	return entity.StorageExhaustedError{
		SizeBytes:   sizeBytes,
		StoredBytes: ledger.StoredBytes,
		MaxBytes:    ledger.MaxBytes,
	}
}

func (s *attachmentsService) Finalize(
	ctx context.Context,
	workspaceID, issueID, attachmentID uuid.UUID,
) (entity.Attachment, error) {
	decision, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
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

	if object.Size != reserved.SizeBytes {
		return entity.Attachment{}, s.refuse(ctx, reserved, entity.AttachmentSizeMismatchError{
			DeclaredBytes: reserved.SizeBytes,
			ArrivedBytes:  object.Size,
		})
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.attachments.Settle(ctx, attachmentID, object.Size, served, time.Now().UTC()); err != nil {
			return err
		}

		return s.record(ctx, decision, reserved, entity.ActivityKindAttachmentAdded)
	}); err != nil {
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

		return s.attachments.Discard(ctx, reserved.ID, reserved.SizeBytes, now)
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
	decision, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return err
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		attachment, err := s.attachments.LockByID(ctx, workspaceID, attachmentID)
		if err != nil {
			return err
		}

		if attachment.IssueID != issueID {
			return entity.ErrAttachmentNotFound
		}

		if err := s.attachments.Release(ctx, workspaceID, attachment.SizeBytes); err != nil {
			return err
		}

		if err := s.attachments.Discard(ctx, attachmentID, attachment.SizeBytes, time.Now().UTC()); err != nil {
			return err
		}

		return s.record(ctx, decision, attachment, entity.ActivityKindAttachmentRemoved)
	}); err != nil {
		return err
	}

	s.kick(ctx)

	return nil
}

func (s *attachmentsService) record(
	ctx context.Context,
	decision entity.Decision,
	attachment entity.Attachment,
	kind entity.ActivityKind,
) error {
	return s.activity.Record(ctx, entity.Activity{
		WorkspaceID: attachment.WorkspaceID,
		Subject:     entity.IssueSubject(attachment.IssueID),
		Actor:       decision.ActivityActor(),
		Kind:        kind,
		Field:       entity.ActivityFieldAttachment,
		ToValue:     attachment.FileName,
	})
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

	if err := s.issues.VisibleExists(ctx, workspaceID, attachment.IssueID, decision.Scope); err != nil {
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

	return s.attachments.Ledger(ctx, workspaceID, s.cfg.MaxWorkspaceBytes)
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

	unsettled, err := s.attachments.ListUnsettledImportFiles(ctx, now, s.cfg.ReclaimBatch)
	if err != nil {
		return err
	}

	for _, file := range unsettled {
		if err := s.settleImportFile(ctx, file.WorkspaceID, file.ObjectKey, 0); err != nil {
			failures++

			logging.From(ctx).WarnContext(
				ctx, "measuring an import file failed",
				"object_key", file.ObjectKey,
				"error", err.Error(),
			)
		}
	}

	adopted, err := s.attachments.ListUnsettledAttachments(ctx, now, s.cfg.ReclaimBatch)
	if err != nil {
		return err
	}

	for _, attachment := range adopted {
		if err := s.settleAttachment(ctx, attachment.WorkspaceID, attachment.ID); err != nil {
			failures++

			logging.From(ctx).WarnContext(
				ctx, "measuring an adopted file failed",
				"attachment_id", attachment.ID.String(),
				"error", err.Error(),
			)
		}
	}

	if failures > 0 {
		return fmt.Errorf(
			"reclaim or measure %d of %d stored files", failures, len(reclaimable)+len(unsettled)+len(adopted),
		)
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

func (s *attachmentsService) ChargeImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
	sizeBytes int64,
) (int64, error) {
	if !importKeyOf(workspaceID, objectKey) {
		return 0, entity.NewValidationError(entity.FieldError{Field: "objectKey", Code: entity.ValidationCodeMalformed})
	}

	if sizeBytes < 0 {
		return 0, entity.NewValidationError(entity.FieldError{Field: "sizeBytes", Code: entity.ValidationCodeMalformed})
	}

	var previous int64

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		file, err := s.attachments.ClaimImportFile(ctx, workspaceID, objectKey)
		if err != nil {
			return err
		}

		adopter, adopted, err := s.adopterOf(ctx, workspaceID, objectKey, file)
		if err != nil {
			return err
		}

		if adopted {
			previous = adopter.SizeBytes

			return s.chargeAttachment(ctx, adopter, sizeBytes)
		}

		previous = file.SizeBytes

		if grown := sizeBytes - file.SizeBytes; grown > 0 {
			if _, err := s.attachments.Admit(ctx, workspaceID, grown, s.cfg.MaxWorkspaceBytes); err != nil {
				return s.exhausted(ctx, workspaceID, grown, err)
			}
		}

		chargedUntil := laterOf(file.ChargedUntil, time.Now().UTC().Add(s.cfg.UploadTTL+abandonGrace))
		settleAfter := soonerOf(file.SettleAfter, chargedUntil)

		return s.attachments.RecordImportFile(
			ctx, workspaceID, objectKey, max(file.SizeBytes, sizeBytes), &settleAfter, &chargedUntil,
		)
	}); err != nil {
		return 0, err
	}

	return previous, nil
}

func (s *attachmentsService) SettleImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
) error {
	if !importKeyOf(workspaceID, objectKey) {
		return malformedImportKey()
	}

	return s.settleImportFile(ctx, workspaceID, objectKey, 0)
}

func (s *attachmentsService) RestoreImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
	previousBytes int64,
) error {
	if !importKeyOf(workspaceID, objectKey) {
		return malformedImportKey()
	}

	return s.settleImportFile(ctx, workspaceID, objectKey, previousBytes)
}

func (s *attachmentsService) settleImportFile(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
	floorBytes int64,
) error {
	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		file, err := s.attachments.ClaimImportFile(ctx, workspaceID, objectKey)
		if err != nil {
			return err
		}

		adopter, adopted, err := s.adopterOf(ctx, workspaceID, objectKey, file)
		if err != nil {
			return err
		}

		if adopted {
			return s.measureAttachment(ctx, adopter, floorBytes)
		}

		sizeBytes, settleAfter, gone := s.measure(ctx, objectKey, file.SizeBytes, floorBytes, file.ChargedUntil)
		if gone {
			return s.attachments.RefundImportFile(ctx, workspaceID, objectKey)
		}

		return s.resizeImportFile(ctx, file, sizeBytes, settleAfter)
	})
}

func (s *attachmentsService) settleAttachment(ctx context.Context, workspaceID, attachmentID uuid.UUID) error {
	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		attachment, err := s.attachments.LockByID(ctx, workspaceID, attachmentID)
		if errors.Is(err, entity.ErrAttachmentNotFound) {
			return nil
		}

		if err != nil {
			return err
		}

		if !attachment.Stored() || attachment.SettleAfter == nil {
			return nil
		}

		return s.measureAttachment(ctx, attachment, 0)
	})
}

func (s *attachmentsService) measureAttachment(
	ctx context.Context,
	attachment entity.Attachment,
	floorBytes int64,
) error {
	sizeBytes, settleAfter, _ := s.measure(
		ctx, attachment.ObjectKey, attachment.SizeBytes, floorBytes, attachment.ChargedUntil,
	)

	if drift := sizeBytes - attachment.SizeBytes; drift != 0 {
		if err := s.attachments.Correct(ctx, attachment.WorkspaceID, drift); err != nil {
			return err
		}
	}

	chargedUntil := attachment.ChargedUntil
	if settleAfter == nil {
		chargedUntil = nil
	}

	return s.attachments.MeasureAttachment(ctx, attachment.ID, sizeBytes, settleAfter, chargedUntil)
}

func (s *attachmentsService) adopterOf(
	ctx context.Context,
	workspaceID uuid.UUID,
	objectKey string,
	file entity.ImportFileCharge,
) (entity.Attachment, bool, error) {
	adopter, adopted, err := s.attachments.LockStoredByObjectKey(ctx, workspaceID, objectKey)
	if err != nil || !adopted {
		return entity.Attachment{}, false, err
	}

	if _, _, err := s.attachments.TakeImportFile(ctx, workspaceID, objectKey); err != nil {
		return entity.Attachment{}, false, err
	}

	reserved := max(adopter.SizeBytes, file.SizeBytes)

	if drift := reserved - adopter.SizeBytes - file.SizeBytes; drift != 0 {
		if err := s.attachments.Correct(ctx, workspaceID, drift); err != nil {
			return entity.Attachment{}, false, err
		}
	}

	adopter.SizeBytes = reserved
	adopter.ChargedUntil = latestOf(adopter.ChargedUntil, file.ChargedUntil)
	adopter.SettleAfter = earliestOf(adopter.SettleAfter, file.SettleAfter)

	return adopter, true, nil
}

func (s *attachmentsService) chargeAttachment(
	ctx context.Context,
	adopter entity.Attachment,
	sizeBytes int64,
) error {
	if grown := sizeBytes - adopter.SizeBytes; grown > 0 {
		if _, err := s.attachments.Admit(ctx, adopter.WorkspaceID, grown, s.cfg.MaxWorkspaceBytes); err != nil {
			return s.exhausted(ctx, adopter.WorkspaceID, grown, err)
		}
	}

	chargedUntil := laterOf(adopter.ChargedUntil, time.Now().UTC().Add(s.cfg.UploadTTL+abandonGrace))
	settleAfter := soonerOf(adopter.SettleAfter, chargedUntil)

	return s.attachments.MeasureAttachment(
		ctx, adopter.ID, max(adopter.SizeBytes, sizeBytes), &settleAfter, &chargedUntil,
	)
}

func latestOf(first, second *time.Time) *time.Time {
	if first == nil || (second != nil && second.After(*first)) {
		return second
	}

	return first
}

func earliestOf(first, second *time.Time) *time.Time {
	if first == nil || (second != nil && second.Before(*first)) {
		return second
	}

	return first
}

func (s *attachmentsService) measure(
	ctx context.Context,
	objectKey string,
	reservedBytes, floorBytes int64,
	chargedUntil *time.Time,
) (int64, *time.Time, bool) {
	now := time.Now().UTC()
	stillWriting := chargedUntil != nil && now.Before(*chargedUntil)

	object, err := s.blobs.Stat(ctx, objectKey)

	switch {
	case err == nil && stillWriting:
		return max(reservedBytes, object.Size), chargedUntil, false
	case err == nil:
		return object.Size, nil, false
	case errors.Is(err, entity.ErrBlobNotFound) && stillWriting:
		return reservedBytes, chargedUntil, false
	case errors.Is(err, entity.ErrBlobNotFound):
		return 0, nil, true
	}

	logging.From(ctx).WarnContext(
		ctx, "a stored file could not be measured, so it stays charged at its larger size until the sweep measures it",
		"object_key", objectKey,
		"error", err.Error(),
	)

	return max(reservedBytes, floorBytes), &now, false
}

func (s *attachmentsService) resizeImportFile(
	ctx context.Context,
	file entity.ImportFileCharge,
	sizeBytes int64,
	settleAfter *time.Time,
) error {
	if drift := sizeBytes - file.SizeBytes; drift != 0 {
		if err := s.attachments.Correct(ctx, file.WorkspaceID, drift); err != nil {
			return err
		}
	}

	chargedUntil := file.ChargedUntil
	if settleAfter == nil {
		chargedUntil = nil
	}

	return s.attachments.RecordImportFile(ctx, file.WorkspaceID, file.ObjectKey, sizeBytes, settleAfter, chargedUntil)
}

func laterOf(current *time.Time, candidate time.Time) time.Time {
	if current != nil && current.After(candidate) {
		return *current
	}

	return candidate
}

func soonerOf(current *time.Time, candidate time.Time) time.Time {
	if current != nil && current.Before(candidate) {
		return *current
	}

	return candidate
}

func malformedImportKey() error {
	return entity.NewValidationError(entity.FieldError{Field: "objectKey", Code: entity.ValidationCodeMalformed})
}

func importKeyOf(workspaceID uuid.UUID, key string) bool {
	return entity.ValidBlobKey(key) && strings.HasPrefix(key, entity.ImportBlobPrefix(workspaceID)+"/")
}

func (s *attachmentsService) kick(ctx context.Context) {
	if err := s.jobs.EnqueueAttachmentReclaim(ctx); err != nil {
		logging.From(ctx).WarnContext(ctx, "scheduling a storage sweep failed", "error", err.Error())
	}
}
