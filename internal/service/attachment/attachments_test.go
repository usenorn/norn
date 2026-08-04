package attachment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	attachmentrepo "github.com/usenorn/norn/internal/repository/attachment"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	attachmentsvc "github.com/usenorn/norn/internal/service/attachment"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
)

const (
	maxFileBytes      = 1_000
	maxWorkspaceBytes = 10_000
)

type harness struct {
	attachments *attachmentrepo.MockAttachment
	issues      *issuerepo.MockIssue
	blobs       *blobrepo.MockBlob
	jobs        *jobqueuerepo.MockJobProducer
	authorizer  *authorizersvc.MockAuthorizer
	service     service.Attachments

	workspaceID  uuid.UUID
	issueID      uuid.UUID
	attachmentID uuid.UUID
	accountID    uuid.UUID
}

func newHarness(t *testing.T, workspaceCap int64) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		attachments:  attachmentrepo.NewMockAttachment(ctrl),
		issues:       issuerepo.NewMockIssue(ctrl),
		blobs:        blobrepo.NewMockBlob(ctrl),
		jobs:         jobqueuerepo.NewMockJobProducer(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID:  uuid.New(),
		issueID:      uuid.New(),
		attachmentID: uuid.New(),
		accountID:    uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.jobs.EXPECT().EnqueueAttachmentReclaim(gomock.Any()).Return(nil).AnyTimes()

	h.service = attachmentsvc.New(
		h.attachments, h.issues, h.blobs, h.jobs, h.authorizer, transactor,
		config.Attachments{
			MaxFileBytes:      maxFileBytes,
			MaxWorkspaceBytes: workspaceCap,
			UploadTTL:         15 * time.Minute,
			LinkTTL:           5 * time.Minute,
			ReclaimBatch:      100,
		},
	)

	return h
}

func (h *harness) actAs(role entity.MembershipRole) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.accountID},
			Role:  role,
			Scope: entity.TeamScope{WorkspaceID: h.workspaceID, AllTeams: true},
		}, nil).
		AnyTimes()
}

func (h *harness) seesTheIssue() {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{ID: h.issueID, WorkspaceID: h.workspaceID}, nil).
		AnyTimes()
}

func (h *harness) cannotSeeTheIssue() {
	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Issue{}, entity.ErrIssueNotFound).
		AnyTimes()
}

func (h *harness) stored(contentType string) entity.Attachment {
	return entity.Attachment{
		ID:          h.attachmentID,
		WorkspaceID: h.workspaceID,
		IssueID:     h.issueID,
		ObjectKey:   entity.AttachmentKey(h.workspaceID, h.attachmentID),
		FileName:    "shot.png",
		ContentType: contentType,
		SizeBytes:   500,
		Status:      entity.AttachmentStatusStored,
	}
}

func (h *harness) reserved() entity.Attachment {
	pending := h.stored(entity.AttachmentGenericType)
	pending.Status = entity.AttachmentStatusPending

	return pending
}

func TestAFileOverThePerFileCapIsRefusedBeforeAnyBytesMove(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	_, err := h.service.Reserve(
		context.Background(), h.workspaceID, h.issueID,
		service.ReserveAttachmentInput{FileName: "huge.bin", SizeBytes: maxFileBytes + 1},
	)

	var oversized entity.AttachmentTooLargeError
	if !errors.As(err, &oversized) {
		t.Fatalf(
			"an oversized file returned %v. Nothing was admitted and no ticket was minted, so the "+
				"refusal has to happen here rather than after the bytes are already stored.",
			err,
		)
	}

	if oversized.MaxBytes != maxFileBytes {
		t.Fatalf("the refusal reported a cap of %d, want %d", oversized.MaxBytes, maxFileBytes)
	}
}

func TestAFullWorkspaceIsRefusedWithItsOwnNumbers(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(500), int64(maxWorkspaceBytes)).
		Return(int64(0), entity.ErrStorageExhausted)
	h.attachments.EXPECT().
		Ledger(gomock.Any(), h.workspaceID).
		Return(entity.WorkspaceStorage{StoredBytes: 9_800}, nil)

	_, err := h.service.Reserve(
		context.Background(), h.workspaceID, h.issueID,
		service.ReserveAttachmentInput{FileName: "shot.png", SizeBytes: 500},
	)

	var exhausted entity.StorageExhaustedError
	if !errors.As(err, &exhausted) {
		t.Fatalf("a full workspace returned %v, which does not tell the uploader what happened", err)
	}

	if exhausted.StoredBytes != 9_800 || exhausted.MaxBytes != maxWorkspaceBytes {
		t.Fatalf(
			"the refusal reported %d of %d. The screen has to be able to say how full the "+
				"workspace is, not merely that it is full.",
			exhausted.StoredBytes, exhausted.MaxBytes,
		)
	}
}

func TestRoomIsTakenBeforeTheRowExistsSoTwoUploadsCannotBothFit(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	admitted := false

	h.attachments.EXPECT().
		Admit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, int64, int64) (int64, error) {
			admitted = true

			return 500, nil
		})
	h.attachments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reserved entity.Attachment) (entity.Attachment, error) {
			if !admitted {
				t.Fatal(
					"the attachment row was written before the workspace was charged for it. " +
						"The admission is what serialises concurrent uploads against the cap.",
				)
			}

			if reserved.Status != entity.AttachmentStatusPending {
				t.Fatalf("the reservation was created as %q rather than pending", reserved.Status)
			}

			if reserved.ReclaimAfter == nil {
				t.Fatal(
					"the reservation has no reclaim deadline, so a client that walks away holds " +
						"that room forever",
				)
			}

			return reserved, nil
		})
	h.blobs.EXPECT().
		PresignPut(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.BlobTicket{URL: "https://storage.example/put", Method: "PUT"}, nil)

	reservation, err := h.service.Reserve(
		context.Background(), h.workspaceID, h.issueID,
		service.ReserveAttachmentInput{FileName: "shot.png", SizeBytes: 500},
	)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	if reservation.Transfer.URL == "" || reservation.Transfer.Method == "" {
		t.Fatal("the reservation did not describe where to send the bytes")
	}
}

func TestFinalizeTrustsWhatItSniffsRatherThanWhatWasDeclared(t *testing.T) {
	for name, sniffed := range map[string]string{
		"a page claiming to be a png": "text/html; charset=utf-8",
		"an svg claiming to be a png": "image/svg+xml",
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, maxWorkspaceBytes)
			h.actAs(entity.MembershipRoleMember)
			h.seesTheIssue()

			h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(h.reserved(), nil)
			h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).
				Return(entity.BlobObject{Size: 500}, nil)
			h.blobs.EXPECT().Sniff(gomock.Any(), gomock.Any()).Return(sniffed, nil)

			var settled string

			h.attachments.EXPECT().
				Settle(gomock.Any(), h.attachmentID, int64(500), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ uuid.UUID, _ int64, contentType string, _ time.Time) error {
					settled = contentType

					return nil
				})
			h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(h.stored(entity.AttachmentGenericType), nil)

			if _, err := h.service.Finalize(
				context.Background(), h.workspaceID, h.issueID, h.attachmentID,
			); err != nil {
				t.Fatalf("Finalize: %v", err)
			}

			if settled != entity.AttachmentGenericType {
				t.Fatalf(
					"a file that sniffs as %q was settled to be served as %q. The declared type "+
						"is a courtesy; what the bytes actually are decides how they go out.",
					sniffed, settled,
				)
			}
		})
	}
}

func TestFinalizeChargesTheDifferenceWhenMoreArrivedThanWasReserved(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(h.reserved(), nil)
	h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).Return(entity.BlobObject{Size: 700}, nil)
	h.blobs.EXPECT().Sniff(gomock.Any(), gomock.Any()).Return("image/png", nil)
	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(200), int64(maxWorkspaceBytes)).
		Return(int64(700), nil)
	h.attachments.EXPECT().
		Settle(gomock.Any(), h.attachmentID, int64(700), "image/png", gomock.Any()).
		Return(nil)
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.stored("image/png"), nil)

	if _, err := h.service.Finalize(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
}

func TestFinalizeGivesBackTheDifferenceWhenLessArrived(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(h.reserved(), nil)
	h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).Return(entity.BlobObject{Size: 300}, nil)
	h.blobs.EXPECT().Sniff(gomock.Any(), gomock.Any()).Return("image/png", nil)
	h.attachments.EXPECT().Release(gomock.Any(), h.workspaceID, int64(200)).Return(nil)
	h.attachments.EXPECT().
		Settle(gomock.Any(), h.attachmentID, int64(300), "image/png", gomock.Any()).
		Return(nil)
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.stored("image/png"), nil)

	if _, err := h.service.Finalize(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); err != nil {
		t.Fatalf(
			"Finalize: %v. A reservation is the declared size; the room a smaller file did not "+
				"use has to come back or the workspace slowly fills with nothing.",
			err,
		)
	}
}

func TestAFileThatArrivesOverTheCapIsDiscardedAndItsRoomReturned(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(h.reserved(), nil)
	h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).
		Return(entity.BlobObject{Size: maxFileBytes + 1}, nil)
	h.blobs.EXPECT().Sniff(gomock.Any(), gomock.Any()).Return("image/png", nil)
	h.attachments.EXPECT().Release(gomock.Any(), h.workspaceID, int64(500)).Return(nil)
	h.attachments.EXPECT().Discard(gomock.Any(), h.attachmentID, gomock.Any()).Return(nil)

	_, err := h.service.Finalize(context.Background(), h.workspaceID, h.issueID, h.attachmentID)

	var oversized entity.AttachmentTooLargeError
	if !errors.As(err, &oversized) {
		t.Fatalf(
			"a presigned PUT cannot bound the body, so a client can always send more than it "+
				"declared. Finalize returned %v rather than refusing it.",
			err,
		)
	}
}

func TestFinalizingTwiceIsRefusedRatherThanChargedTwice(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.stored("image/png"), nil)

	if _, err := h.service.Finalize(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); !errors.Is(err, entity.ErrAttachmentNotPending) {
		t.Fatalf("finalizing a settled attachment returned %v", err)
	}
}

func TestFinalizingSomethingNobodyUploadedSaysSoRatherThanStoringAZeroByteFile(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(h.reserved(), nil)
	h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).Return(entity.BlobObject{}, entity.ErrBlobNotFound)

	if _, err := h.service.Finalize(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); !errors.Is(err, entity.ErrAttachmentMissing) {
		t.Fatalf("finalizing without an upload returned %v", err)
	}
}

func TestAFileOnAnIssueYouCannotSeeIsIndistinguishableFromOneThatDoesNotExist(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.cannotSeeTheIssue()
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.stored("image/png"), nil)

	if _, err := h.service.Content(
		context.Background(), h.workspaceID, h.attachmentID,
	); !errors.Is(err, entity.ErrIssueNotFound) {
		t.Fatalf(
			"downloading a file whose issue is invisible returned %v. The link is resolved "+
				"through the issue on every fetch precisely so a URL is never a way around it.",
			err,
		)
	}
}

func TestAFileThatHasNotSettledHasNoBytesToHandOut(t *testing.T) {
	for name, attachment := range map[string]func(*harness) entity.Attachment{
		"still pending": func(h *harness) entity.Attachment { return h.reserved() },
		"orphaned": func(h *harness) entity.Attachment {
			orphan := h.stored("image/png")
			orphan.IssueID = uuid.Nil

			return orphan
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, maxWorkspaceBytes)
			h.actAs(entity.MembershipRoleMember)
			h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(attachment(h), nil)

			if _, err := h.service.Content(
				context.Background(), h.workspaceID, h.attachmentID,
			); !errors.Is(err, entity.ErrAttachmentNotFound) {
				t.Fatalf("a %s attachment handed out a link: %v", name, err)
			}
		})
	}
}

func TestTheLinkCarriesTheServedTypeAndDispositionRatherThanTheFileName(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	dangerous := h.stored(entity.AttachmentGenericType)
	dangerous.FileName = "trap.svg"
	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(dangerous, nil)

	var served entity.ServeSpec

	h.blobs.EXPECT().
		PresignGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, spec entity.ServeSpec, _ time.Duration) (string, error) {
			served = spec

			return "https://storage.example/get", nil
		})

	if _, err := h.service.Content(context.Background(), h.workspaceID, h.attachmentID); err != nil {
		t.Fatalf("Content: %v", err)
	}

	if served.Disposition != entity.AttachmentDispositionAt {
		t.Fatalf(
			"a file named .svg would be served %q. The extension in the name decides nothing; "+
				"the settled content type does.",
			served.Disposition,
		)
	}
}

func TestRemovingAFileGivesItsRoomBackAtOnce(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(h.stored("image/png"), nil)
	h.attachments.EXPECT().Release(gomock.Any(), h.workspaceID, int64(500)).Return(nil)
	h.attachments.EXPECT().Discard(gomock.Any(), h.attachmentID, gomock.Any()).Return(nil)

	if err := h.service.Remove(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestReclaimRemovesTheObjectBeforeItForgetsWhereItWas(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)

	orphan := h.stored("image/png")
	deleted := false

	h.attachments.EXPECT().MarkOrphans(gomock.Any(), gomock.Any()).Return(nil)
	h.attachments.EXPECT().ListReclaimable(gomock.Any(), gomock.Any(), 100).
		Return([]entity.Attachment{orphan}, nil)
	h.blobs.EXPECT().
		Delete(gomock.Any(), orphan.ObjectKey).
		DoAndReturn(func(context.Context, string) error {
			deleted = true

			return nil
		})
	h.attachments.EXPECT().
		Reclaim(gomock.Any(), orphan.ID).
		DoAndReturn(func(context.Context, uuid.UUID) error {
			if !deleted {
				t.Fatal(
					"the row was reclaimed before the object was deleted. The row holds the only " +
						"copy of the key, so the bytes would be stranded and unbilled.",
				)
			}

			return nil
		})

	if err := h.service.Reclaim(context.Background()); err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
}

func TestReclaimReportsAFailureRatherThanLookingLikeAHealthySweep(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)

	h.attachments.EXPECT().MarkOrphans(gomock.Any(), gomock.Any()).Return(nil)
	h.attachments.EXPECT().ListReclaimable(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entity.Attachment{h.stored("image/png")}, nil)
	h.blobs.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(errors.New("storage is unreachable"))

	if err := h.service.Reclaim(context.Background()); err == nil {
		t.Fatal(
			"a sweep that could not reach storage returned success. Asynq would then never " +
				"retry it and a total outage would be invisible in the jobs surface.",
		)
	}
}

func TestOnlySomeoneWhoMayChangeTheWorkspaceSeesWhatItIsStoring(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)

	var asked entity.AccessRequest

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			asked = request

			return entity.Decision{Role: entity.MembershipRoleAdmin}, nil
		})
	h.attachments.EXPECT().Ledger(gomock.Any(), h.workspaceID).
		Return(entity.WorkspaceStorage{StoredBytes: 2_048}, nil)

	ledger, err := h.service.Ledger(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}

	if asked.Resource != entity.ResourceWorkspace || asked.Action != entity.ActionUpdate {
		t.Fatalf(
			"the consumption figure was authorised as %s/%s. Only a workspace change is "+
				"admin-only; anything a member may do would put a workspace-wide total in front "+
				"of someone who can subtract it from what they can see.",
			asked.Resource, asked.Action,
		)
	}

	if ledger.MaxBytes != maxWorkspaceBytes || ledger.StoredBytes != 2_048 {
		t.Fatalf("the figure read %d of %d", ledger.StoredBytes, ledger.MaxBytes)
	}
}

func TestAnUnlimitedWorkspaceStillReportsWhatItIsStoring(t *testing.T) {
	h := newHarness(t, 0)
	h.authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(entity.Decision{}, nil)
	h.attachments.EXPECT().Ledger(gomock.Any(), gomock.Any()).
		Return(entity.WorkspaceStorage{StoredBytes: 4_096}, nil)

	ledger, err := h.service.Ledger(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}

	if !ledger.Unlimited() || ledger.StoredBytes != 4_096 {
		t.Fatalf(
			"an unconfigured cap reported %d of %d. A self-hoster who set no quota is still "+
				"metered; they simply are not stopped.",
			ledger.StoredBytes, ledger.MaxBytes,
		)
	}
}
