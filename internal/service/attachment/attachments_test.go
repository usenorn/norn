package attachment_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
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
	activity    *activityrepo.MockActivity
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
		activity:     activityrepo.NewMockActivity(ctrl),
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
	h.activity.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.service = attachmentsvc.New(
		h.attachments, h.activity, h.issues, h.blobs, h.jobs, h.authorizer, transactor,
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
		VisibleExists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
}

func (h *harness) cannotSeeTheIssue() {
	h.issues.EXPECT().
		VisibleExists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.ErrIssueNotFound).
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
		Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 9_800, MaxBytes: maxWorkspaceBytes}, nil)

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
		PresignPut(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

func (h *harness) carried(origin *entity.ImportOrigin) service.AdoptAttachmentInput {
	return service.AdoptAttachmentInput{
		ObjectKey:   entity.ImportBlobKey(h.workspaceID, uuid.New(), "shot.png"),
		FileName:    "shot.png",
		ContentType: "image/png",
		SizeBytes:   500,
		Origin:      origin,
	}
}

func decodedOrigin(t *testing.T, author uuid.UUID) *entity.ImportOrigin {
	t.Helper()

	body := `{"CreatedAt":"2021-03-04T05:06:07Z","UpdatedAt":"2021-03-04T05:06:07Z",` +
		`"AuthorAccountID":"` + author.String() + `"}`

	var decoded entity.ImportOrigin

	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("unmarshal origin: %v", err)
	}

	return &decoded
}

func TestAdoptingAnObjectAsksWhetherTheOriginIsAttributedRatherThanPresent(t *testing.T) {
	for name, origin := range map[string]func(*testing.T, *harness) *entity.ImportOrigin{
		"no origin at all": func(*testing.T, *harness) *entity.ImportOrigin { return nil },
		"one decoded from a request body": func(t *testing.T, h *harness) *entity.ImportOrigin {
			return decodedOrigin(t, h.accountID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, maxWorkspaceBytes)
			h.actAs(entity.MembershipRoleMember)
			h.seesTheIssue()

			_, err := h.service.Adopt(
				context.Background(), h.workspaceID, h.issueID, h.carried(origin(t, h)),
			)

			if !errors.Is(err, entity.ErrAttachmentAdoptNeedsOrigin) {
				t.Fatalf(
					"adopting with %s returned %v. Adopt writes a row that is stored on arrival and "+
						"never passes the sweep, so it is reachable only by an import: an origin the "+
						"constructor never made is inert however its fields are filled, and testing "+
						"the pointer for presence would hand that to anyone who named the field.",
					name, err,
				)
			}
		})
	}
}

func TestAnAdoptedFileIsStoredOnItsIssueBeforeAnySweepCouldSeeIt(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
	origin := entity.NewImportOrigin(source, source, h.accountID)

	h.attachments.EXPECT().
		TakeImportFile(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(int64(0), false, nil)
	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(500), int64(maxWorkspaceBytes)).
		Return(int64(500), nil)
	h.attachments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, adopted entity.Attachment) (entity.Attachment, error) {
			if adopted.Status != entity.AttachmentStatusStored {
				t.Fatalf(
					"the row was created %q. The bytes are already in storage, and a row that has "+
						"to be settled afterwards means a crash mid-import leaves a file nothing owns.",
					adopted.Status,
				)
			}

			if adopted.IssueID != h.issueID || adopted.ReclaimAfter != nil {
				t.Fatalf(
					"the row was created with issue %v and reclaim deadline %v. MarkOrphans discards "+
						"everything where reclaim_after IS NULL AND issue_id IS NULL, so either of "+
						"those wrong deletes a live import's file on the next five-minute sweep.",
					adopted.IssueID, adopted.ReclaimAfter,
				)
			}

			createdAt, updatedAt := entity.OriginStamp(adopted.Origin, time.Now().UTC())
			if !createdAt.Equal(source) || !updatedAt.Equal(source) {
				t.Fatalf(
					"the row would be stamped %s/%s rather than %s. A file carried across keeps the "+
						"time it was attached at the source, or the issue's history reads as though "+
						"every attachment arrived on migration day.",
					createdAt, updatedAt, source,
				)
			}

			return adopted, nil
		})

	adopted := h.carried(&origin)

	if _, err := h.service.Adopt(
		context.Background(), h.workspaceID, h.issueID, adopted,
	); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
}

func TestAnImportsFileIsRefusedByAFullWorkspaceLikeAnyOtherUpload(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
	origin := entity.NewImportOrigin(source, source, h.accountID)

	h.attachments.EXPECT().
		TakeImportFile(gomock.Any(), h.workspaceID, gomock.Any()).
		Return(int64(0), false, nil)
	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(500), int64(maxWorkspaceBytes)).
		Return(int64(0), entity.ErrStorageExhausted)
	h.attachments.EXPECT().
		Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 9_900, MaxBytes: maxWorkspaceBytes}, nil)

	_, err := h.service.Adopt(context.Background(), h.workspaceID, h.issueID, h.carried(&origin))

	if !errors.Is(err, entity.ErrStorageExhausted) {
		t.Fatalf(
			"a file carried into a full workspace returned %v. An import is unbounded in how many "+
				"records it may carry, but its files are charged against the same quota as anything "+
				"else, and the run has to be told to stop rather than to keep going and be dropped.",
			err,
		)
	}
}

func TestAKeyThisInstanceCouldNotHaveMintedIsNeverAdopted(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
	origin := entity.NewImportOrigin(source, source, h.accountID)

	carried := h.carried(&origin)
	carried.ObjectKey = "imports/../attachments/somebody-elses-workspace/secret"

	var refused entity.ValidationError

	if _, err := h.service.Adopt(
		context.Background(), h.workspaceID, h.issueID, carried,
	); !errors.As(err, &refused) {
		t.Fatalf(
			"a key that climbs out of its prefix was adopted with %v. Adopt is the one place a key "+
				"arrives from a caller rather than being minted here, so the guard that has never "+
				"had to fire is exactly the one that has to hold now.",
			err,
		)
	}
}

func TestAnAdoptedFileIsServedByWhatItIsRatherThanWhatTheSourceSaid(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
	origin := entity.NewImportOrigin(source, source, h.accountID)

	carried := h.carried(&origin)
	carried.ContentType = "image/svg+xml"

	h.attachments.EXPECT().TakeImportFile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(0), false, nil)
	h.attachments.EXPECT().Admit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(int64(500), nil)
	h.attachments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, adopted entity.Attachment) (entity.Attachment, error) {
			if adopted.ContentType != entity.AttachmentGenericType {
				t.Fatalf(
					"a file the source described as %q would be served as %q. The type on the row "+
						"decides the disposition of every later download, and a type another system "+
						"asserted is no more trustworthy than one a browser declared.",
					carried.ContentType, adopted.ContentType,
				)
			}

			return adopted, nil
		})

	if _, err := h.service.Adopt(
		context.Background(), h.workspaceID, h.issueID, carried,
	); err != nil {
		t.Fatalf("Adopt: %v", err)
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

func TestFinalizeRefusesAFileThatIsNotTheSizeItWasReservedAt(t *testing.T) {
	for name, arrived := range map[string]int64{
		"more than was declared": 700,
		"less than was declared": 300,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, maxWorkspaceBytes)
			h.actAs(entity.MembershipRoleMember)
			h.seesTheIssue()

			h.attachments.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(h.reserved(), nil)
			h.blobs.EXPECT().Stat(gomock.Any(), gomock.Any()).Return(entity.BlobObject{Size: arrived}, nil)
			h.blobs.EXPECT().Sniff(gomock.Any(), gomock.Any()).Return("image/png", nil)
			h.attachments.EXPECT().Release(gomock.Any(), h.workspaceID, int64(500)).Return(nil)
			h.attachments.EXPECT().Discard(gomock.Any(), h.attachmentID, int64(500), gomock.Any()).Return(nil)

			_, err := h.service.Finalize(context.Background(), h.workspaceID, h.issueID, h.attachmentID)

			var mismatch entity.AttachmentSizeMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf(
					"a file of %d bytes against a reservation of 500 was finalized with %v. The link "+
						"was signed for the declared size, so any other size means the object was "+
						"replaced or cut short, and settling it would bill the wrong number or hand "+
						"out a broken file.",
					arrived, err,
				)
			}

			if mismatch.DeclaredBytes != 500 || mismatch.ArrivedBytes != arrived {
				t.Fatalf(
					"the refusal reported %d declared and %d arrived",
					mismatch.DeclaredBytes, mismatch.ArrivedBytes,
				)
			}
		})
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
	h.attachments.EXPECT().Discard(gomock.Any(), h.attachmentID, int64(500), gomock.Any()).Return(nil)

	_, err := h.service.Finalize(context.Background(), h.workspaceID, h.issueID, h.attachmentID)

	var oversized entity.AttachmentTooLargeError
	if !errors.As(err, &oversized) {
		t.Fatalf(
			"the upload link bounds what a browser sends, but the object is only ever measured "+
				"here. Finalize returned %v rather than refusing a file over the per-file cap.",
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
	h.attachments.EXPECT().Discard(gomock.Any(), h.attachmentID, int64(500), gomock.Any()).Return(nil)

	if err := h.service.Remove(
		context.Background(), h.workspaceID, h.issueID, h.attachmentID,
	); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

func TestAnUploadLinkIsSignedForExactlyTheSizeThatWasReserved(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().Admit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(500), nil)
	h.attachments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, reserved entity.Attachment) (entity.Attachment, error) {
			return reserved, nil
		})

	var signed int64

	h.blobs.EXPECT().
		PresignPut(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, sizeBytes int64, _ time.Duration) (entity.BlobTicket, error) {
			signed = sizeBytes

			return entity.BlobTicket{URL: "https://storage.example/put", Method: "PUT"}, nil
		})

	if _, err := h.service.Reserve(
		context.Background(), h.workspaceID, h.issueID,
		service.ReserveAttachmentInput{FileName: "shot.png", SizeBytes: 500},
	); err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	if signed != 500 {
		t.Fatalf(
			"the upload link was signed for %d bytes against a reservation of 500. The workspace "+
				"was charged for what was declared, so a link that takes any other size lets a "+
				"client store more than it paid room for.",
			signed,
		)
	}
}

func TestAWorkspaceGivenItsOwnLimitReportsThatRatherThanTheInstanceDefault(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.authorizer.EXPECT().Decide(gomock.Any(), gomock.Any()).Return(entity.Decision{}, nil)
	h.attachments.EXPECT().Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 2_048, MaxBytes: 50_000}, nil)

	ledger, err := h.service.Ledger(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}

	if ledger.MaxBytes != 50_000 {
		t.Fatalf(
			"a workspace given 50000 bytes of its own reported a limit of %d. The settings page "+
				"would then show a limit the uploads are not actually held to.",
			ledger.MaxBytes,
		)
	}
}

func TestAFullWorkspaceIsToldTheLimitItIsActuallyHeldTo(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(500), int64(maxWorkspaceBytes)).
		Return(int64(0), entity.ErrStorageExhausted)
	h.attachments.EXPECT().
		Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 49_800, MaxBytes: 50_000}, nil)

	_, err := h.service.Reserve(
		context.Background(), h.workspaceID, h.issueID,
		service.ReserveAttachmentInput{FileName: "shot.png", SizeBytes: 500},
	)

	var exhausted entity.StorageExhaustedError
	if !errors.As(err, &exhausted) || exhausted.MaxBytes != 50_000 {
		t.Fatalf(
			"a workspace held to its own 50000 bytes was refused with %v. Quoting the instance "+
				"default instead would tell the uploader they have room they do not.",
			err,
		)
	}
}

func (h *harness) importKey() string {
	return entity.ImportBlobKey(h.workspaceID, h.attachmentID, "rows.csv")
}

func TestAnImportFileWrittenAgainIsChargedOnlyForWhatItGrew(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	key := h.importKey()

	h.attachments.EXPECT().ClaimImportFile(gomock.Any(), h.workspaceID, key).Return(int64(300), nil)
	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(200), int64(maxWorkspaceBytes)).
		Return(int64(800), nil)
	h.attachments.EXPECT().SizeImportFile(gomock.Any(), h.workspaceID, key, int64(500)).Return(nil)

	previous, err := h.service.ChargeImportFile(context.Background(), h.workspaceID, key, 500)
	if err != nil {
		t.Fatalf(
			"ChargeImportFile: %v. A file uploaded again under the same name replaces the first, "+
				"so charging all 500 bytes would bill the 300 that are no longer stored.",
			err,
		)
	}

	if previous != 300 {
		t.Fatalf(
			"the charge reported the file had held %d bytes, want 300. A write that fails after "+
				"this has only that number to put the file back to.",
			previous,
		)
	}
}

func TestAnImportFileWrittenAgainSmallerGivesBackWhatItShrank(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	key := h.importKey()

	h.attachments.EXPECT().ClaimImportFile(gomock.Any(), h.workspaceID, key).Return(int64(500), nil)
	h.attachments.EXPECT().Release(gomock.Any(), h.workspaceID, int64(300)).Return(nil)
	h.attachments.EXPECT().SizeImportFile(gomock.Any(), h.workspaceID, key, int64(200)).Return(nil)

	if _, err := h.service.ChargeImportFile(context.Background(), h.workspaceID, key, 200); err != nil {
		t.Fatalf("ChargeImportFile: %v", err)
	}
}

func TestAnImportFileIntoAFullWorkspaceIsRefusedWithItsNumbersAndNeverRecorded(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	key := h.importKey()

	h.attachments.EXPECT().ClaimImportFile(gomock.Any(), h.workspaceID, key).Return(int64(0), nil)
	h.attachments.EXPECT().
		Admit(gomock.Any(), h.workspaceID, int64(400), int64(maxWorkspaceBytes)).
		Return(int64(0), entity.ErrStorageExhausted)
	h.attachments.EXPECT().
		Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 9_900, MaxBytes: maxWorkspaceBytes}, nil)

	_, err := h.service.ChargeImportFile(context.Background(), h.workspaceID, key, 400)

	var exhausted entity.StorageExhaustedError
	if !errors.As(err, &exhausted) || exhausted.StoredBytes != 9_900 {
		t.Fatalf(
			"an import file into a full workspace returned %v. The upload has to be refused "+
				"before the bytes are written, and say how full the workspace is.",
			err,
		)
	}
}

func TestAKeyOutsideThisWorkspacesImportsIsNeverCharged(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)

	for name, key := range map[string]string{
		"another workspace's import": entity.ImportBlobKey(uuid.New(), uuid.New(), "rows.csv"),
		"a live attachment":          entity.AttachmentKey(h.workspaceID, uuid.New()),
	} {
		t.Run(name, func(t *testing.T) {
			var refused entity.ValidationError

			if _, err := h.service.ChargeImportFile(
				context.Background(), h.workspaceID, key, 100,
			); !errors.As(err, &refused) {
				t.Fatalf(
					"charging %s returned %v. A key that is not this workspace's own import would "+
						"let one import put its bytes on another workspace's bill.",
					name, err,
				)
			}
		})
	}
}

func TestAnImportedFileAdoptedOntoAnIssueIsNotChargedASecondTime(t *testing.T) {
	h := newHarness(t, maxWorkspaceBytes)
	h.actAs(entity.MembershipRoleMember)
	h.seesTheIssue()

	source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
	origin := entity.NewImportOrigin(source, source, h.accountID)
	carried := h.carried(&origin)

	h.attachments.EXPECT().
		TakeImportFile(gomock.Any(), h.workspaceID, carried.ObjectKey).
		Return(int64(700), true, nil)
	h.attachments.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, adopted entity.Attachment) (entity.Attachment, error) {
			if adopted.SizeBytes != 700 {
				t.Fatalf(
					"the adopted row records %d bytes where the import was charged 700. The sweep "+
						"gives back what the row says, so the two have to agree.",
					adopted.SizeBytes,
				)
			}

			return adopted, nil
		})

	if _, err := h.service.Adopt(context.Background(), h.workspaceID, h.issueID, carried); err != nil {
		t.Fatalf(
			"Adopt: %v. The import already paid for these bytes when it stored them; admitting "+
				"them again would count one file twice.",
			err,
		)
	}
}

type storageBook struct {
	repository.Attachment

	stored int64
	files  map[string]int64
}

func (b *storageBook) ClaimImportFile(_ context.Context, _ uuid.UUID, key string) (int64, error) {
	if _, held := b.files[key]; !held {
		b.files[key] = 0
	}

	return b.files[key], nil
}

func (b *storageBook) SizeImportFile(_ context.Context, _ uuid.UUID, key string, sizeBytes int64) error {
	b.files[key] = sizeBytes

	return nil
}

func (b *storageBook) RefundImportFile(_ context.Context, _ uuid.UUID, key string) error {
	b.stored = max(b.stored-b.files[key], 0)
	delete(b.files, key)

	return nil
}

func (b *storageBook) Admit(_ context.Context, _ uuid.UUID, sizeBytes, maxBytes int64) (int64, error) {
	if maxBytes != 0 && b.stored+sizeBytes > maxBytes {
		return 0, entity.ErrStorageExhausted
	}

	b.stored += sizeBytes

	return b.stored, nil
}

func (b *storageBook) Release(_ context.Context, _ uuid.UUID, sizeBytes int64) error {
	b.stored = max(b.stored-sizeBytes, 0)

	return nil
}

func (b *storageBook) Correct(_ context.Context, _ uuid.UUID, deltaBytes int64) error {
	b.stored = max(b.stored+deltaBytes, 0)

	return nil
}

type measuredStore struct {
	repository.Blob

	objects     map[string]int64
	unreachable bool
}

func (m *measuredStore) Stat(_ context.Context, key string) (entity.BlobObject, error) {
	if m.unreachable {
		return entity.BlobObject{}, errors.New("storage is unreachable")
	}

	size, held := m.objects[key]
	if !held {
		return entity.BlobObject{}, entity.ErrBlobNotFound
	}

	return entity.BlobObject{Size: size}, nil
}

func bookedService(t *testing.T, book *storageBook, store *measuredStore) service.Attachments {
	t.Helper()

	ctrl := gomock.NewController(t)

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	return attachmentsvc.New(
		book, activityrepo.NewMockActivity(ctrl), issuerepo.NewMockIssue(ctrl), store,
		jobqueuerepo.NewMockJobProducer(ctrl), authorizersvc.NewMockAuthorizer(ctrl), transactor,
		config.Attachments{MaxFileBytes: maxFileBytes, MaxWorkspaceBytes: maxWorkspaceBytes},
	)
}

func TestAnImportFileWhoseWriteFailedIsCountedAtWhatStorageStillHolds(t *testing.T) {
	const nothingLeft = -1

	for name, write := range map[string]struct {
		earlier     int64
		left        int64
		unreachable bool
		want        int64
	}{
		"a replacement the store refused, keeping the earlier file":      {300, 300, false, 300},
		"a replacement that landed although the store reported an error": {300, 500, false, 500},
		"a replacement whose outcome cannot be checked":                  {300, 300, true, 300},
		"a first upload that never reached storage":                      {0, nothingLeft, false, 0},
	} {
		t.Run(name, func(t *testing.T) {
			workspaceID := uuid.New()
			key := entity.ImportBlobKey(workspaceID, uuid.New(), "rows.csv")

			book := &storageBook{stored: write.earlier, files: map[string]int64{}}
			store := &measuredStore{objects: map[string]int64{}, unreachable: write.unreachable}

			if write.earlier > 0 {
				book.files[key] = write.earlier
				store.objects[key] = write.earlier
			}

			storage := bookedService(t, book, store)

			previous, err := storage.ChargeImportFile(context.Background(), workspaceID, key, 500)
			if err != nil {
				t.Fatalf("ChargeImportFile: %v", err)
			}

			if book.stored != 500 {
				t.Fatalf("charging a 500 byte upload left the workspace reading %d bytes", book.stored)
			}

			if write.left == nothingLeft {
				delete(store.objects, key)
			} else {
				store.objects[key] = write.left
			}

			if err := storage.RestoreImportFile(context.Background(), workspaceID, key, previous); err != nil {
				t.Fatalf("RestoreImportFile: %v", err)
			}

			recorded, held := book.files[key]

			if book.stored != write.want || recorded != write.want || held != (write.want > 0) {
				t.Fatalf(
					"after %s the file is recorded at %d bytes (held %v) and the workspace reads %d, "+
						"want %d for both. Whatever is still in storage has to stay counted, and "+
						"nothing more: refunding the whole file hides the earlier bytes, and keeping "+
						"the failed charge bills bytes that never arrived.",
					name, recorded, held, book.stored, write.want,
				)
			}
		})
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
	h.attachments.EXPECT().Ledger(gomock.Any(), h.workspaceID, int64(maxWorkspaceBytes)).
		Return(entity.WorkspaceStorage{StoredBytes: 2_048, MaxBytes: maxWorkspaceBytes}, nil)

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
	h.attachments.EXPECT().Ledger(gomock.Any(), gomock.Any(), int64(0)).
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

func TestAKeyPointingOutsideThisWorkspacesOwnImportIsRefused(t *testing.T) {
	other := uuid.New()

	cases := []struct {
		name string
		key  string
	}{
		{
			"another workspace's import",
			entity.ImportBlobKey(other, uuid.New(), "shot.png"),
		},
		{
			"a live attachment of this workspace",
			entity.AttachmentKey(uuid.New(), uuid.New()),
		},
		{
			"the import prefix of another workspace by string alone",
			entity.ImportKeyPrefix + "/" + other.String() + "/run/shot.png",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t, maxWorkspaceBytes)
			h.actAs(entity.MembershipRoleMember)
			h.seesTheIssue()

			source := time.Date(2021, time.March, 4, 5, 6, 7, 0, time.UTC)
			origin := entity.NewImportOrigin(source, source, h.accountID)

			adopted := h.carried(&origin)
			adopted.ObjectKey = testCase.key

			_, err := h.service.Adopt(context.Background(), h.workspaceID, h.issueID, adopted)

			var validation entity.ValidationError

			if !errors.As(err, &validation) {
				t.Fatalf(
					"Adopt accepted the key %q (%v). Adoption takes ownership of whatever object it "+
						"is handed and a later revert deletes it, so a key naming somebody else's "+
						"stored file would have this import claim it and the undo destroy it. An "+
						"adapter is ordinary code that will one day be written by somebody in a "+
						"hurry, and this is the only thing standing between a typo and another "+
						"workspace losing a file.",
					testCase.key, err,
				)
			}
		})
	}
}
