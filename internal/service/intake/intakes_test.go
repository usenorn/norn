package intake_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	inboundmailrepo "github.com/usenorn/norn/internal/repository/inboundmail"
	intakerepo "github.com/usenorn/norn/internal/repository/intake"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	intakesvc "github.com/usenorn/norn/internal/service/intake"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
)

const domain = "submit.norn.so"

type harness struct {
	intake     *intakerepo.MockIntake
	mail       *inboundmailrepo.MockInboundMail
	teams      *teamrepo.MockTeam
	jobs       *jobqueuerepo.MockJobProducer
	issues     *issuesvc.MockIssues
	authorizer *authorizersvc.MockAuthorizer
	service    service.Intakes

	workspaceID uuid.UUID
	teamID      uuid.UUID
	enablerID   uuid.UUID
	deliveryID  uuid.UUID
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		intake:      intakerepo.NewMockIntake(ctrl),
		mail:        inboundmailrepo.NewMockInboundMail(ctrl),
		teams:       teamrepo.NewMockTeam(ctrl),
		jobs:        jobqueuerepo.NewMockJobProducer(ctrl),
		issues:      issuesvc.NewMockIssues(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		teamID:      uuid.New(),
		enablerID:   uuid.New(),
		deliveryID:  uuid.New(),
	}

	h.service = intakesvc.New(
		h.intake, h.mail, h.teams, h.jobs, h.issues, h.authorizer,
		config.Intake{Domain: domain},
	)

	return h
}

func (h *harness) permits(action entity.Action) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			if request.Action != action {
				return entity.Decision{}, entity.AccessDeniedError{}
			}

			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.enablerID},
				Scope: entity.TeamScope{WorkspaceID: h.workspaceID, TeamIDs: []uuid.UUID{h.teamID}},
			}, nil
		})
}

func (h *harness) address() entity.IntakeAddress {
	return entity.IntakeAddress{
		TeamID:      h.teamID,
		WorkspaceID: h.workspaceID,
		LocalPart:   "core-649848208d3e",
		Domain:      domain,
		EnabledBy:   h.enablerID,
		CreatedAt:   time.Date(2026, 2, 11, 9, 0, 0, 0, time.UTC),
	}
}

func (h *harness) delivery() entity.IntakeDelivery {
	return entity.IntakeDelivery{
		ID:          h.deliveryID,
		WorkspaceID: h.workspaceID,
		TeamID:      h.teamID,
		ExternalID:  "inb_1",
		Recipient:   "core-649848208d3e@" + domain,
		Sender:      "rae@northwind.co",
		ReceivedAt:  time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
	}
}

func TestTurningItOnMintsAnAddressNamedAfterTheTeam(t *testing.T) {
	h := newHarness(t)
	h.permits(entity.ActionManage)

	h.intake.EXPECT().
		Address(gomock.Any(), h.workspaceID, h.teamID).
		Return(entity.IntakeAddress{}, entity.ErrIntakeDisabled)
	h.teams.EXPECT().
		GetByID(gomock.Any(), h.teamID).
		Return(entity.Team{ID: h.teamID, WorkspaceID: h.workspaceID, Name: "Core", Key: "DC"}, nil)

	var saved entity.IntakeAddress

	h.intake.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, address entity.IntakeAddress) (entity.IntakeAddress, error) {
			saved = address

			return address, nil
		})

	address, err := h.service.Enable(context.Background(), h.workspaceID, h.teamID)
	if err != nil {
		t.Fatalf("enabling refused: %v", err)
	}

	if saved.EnabledBy != h.enablerID {
		t.Fatalf(
			"the address records %v as its owner, want the person who turned it on. Filing what "+
				"arrives is done in their name, so an address with nobody behind it files nothing.",
			saved.EnabledBy,
		)
	}

	if address.Domain != domain {
		t.Fatalf("address domain=%q, want the instance's own %q", address.Domain, domain)
	}

	if len(saved.LocalPart) <= len("core-") || saved.LocalPart[:5] != "core-" {
		t.Fatalf(
			"local part=%q, want the team's name and a random tail. An address anyone can guess "+
				"from the team name is an open door for spam.",
			saved.LocalPart,
		)
	}
}

func TestTurningItOnTwiceKeepsTheAddressPeopleAlreadyHave(t *testing.T) {
	h := newHarness(t)
	h.permits(entity.ActionManage)

	h.intake.EXPECT().
		Address(gomock.Any(), h.workspaceID, h.teamID).
		Return(h.address(), nil)

	address, err := h.service.Enable(context.Background(), h.workspaceID, h.teamID)
	if err != nil {
		t.Fatalf("enabling refused: %v", err)
	}

	if address.LocalPart != h.address().LocalPart {
		t.Fatalf(
			"the address changed to %q. Turning on something already on must not silently break "+
				"every sender who already has the old address.",
			address.LocalPart,
		)
	}
}

func TestAnInstanceWithNoDomainCannotOfferAnAddress(t *testing.T) {
	h := newHarness(t)

	h.service = intakesvc.New(
		h.intake, h.mail, h.teams, h.jobs, h.issues, h.authorizer, config.Intake{},
	)

	if _, err := h.service.Enable(context.Background(), h.workspaceID, h.teamID); !errors.Is(
		err, entity.ErrIntakeDomainUnset,
	) {
		t.Fatalf(
			"err=%v, want the unset domain. Minting an address on a domain nothing delivers to "+
				"hands the team a dead letterbox.",
			err,
		)
	}
}

func TestMailForAnAddressNobodyClaimsIsNotRecorded(t *testing.T) {
	h := newHarness(t)

	h.mail.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(entity.InboundNotice{DeliveryID: "whd_1", MessageID: "inb_1"}, nil)
	h.mail.EXPECT().
		Fetch(gomock.Any(), "inb_1").
		Return(entity.InboundMessage{
			ExternalID: "inb_1",
			Sender:     "rae@northwind.co",
			Recipients: []string{"nobody-000000000000@" + domain},
		}, nil)
	h.intake.EXPECT().
		AddressOf(gomock.Any(), "nobody-000000000000", domain).
		Return(entity.IntakeAddress{}, entity.ErrIntakeAddressUnknown)

	if _, err := h.service.Accept(context.Background(), http.Header{}, nil); !errors.Is(
		err, entity.ErrIntakeAddressUnknown,
	) {
		t.Fatalf("err=%v, want the unknown address", err)
	}
}

func TestAnEventThatCarriesNoMailIsAcceptedAndDropped(t *testing.T) {
	h := newHarness(t)

	h.mail.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(entity.InboundNotice{DeliveryID: "whd_1"}, nil)

	deliveryID, err := h.service.Accept(context.Background(), http.Header{}, nil)
	if err != nil {
		t.Fatalf("a delivery test event was refused: %v", err)
	}

	if deliveryID != uuid.Nil {
		t.Fatalf(
			"delivery=%v, want nothing recorded. A provider sends test pings and delivery receipts "+
				"down the same webhook, and neither is a report anybody filed.",
			deliveryID,
		)
	}
}

func TestAcceptedMailIsRecordedOnceAndHandedToTheWorker(t *testing.T) {
	h := newHarness(t)

	h.mail.EXPECT().
		Verify(gomock.Any(), gomock.Any()).
		Return(entity.InboundNotice{DeliveryID: "whd_1", MessageID: "inb_1"}, nil)
	h.mail.EXPECT().
		Fetch(gomock.Any(), "inb_1").
		Return(entity.InboundMessage{
			ExternalID: "inb_1",
			Sender:     "Rae Whitfield <rae@northwind.co>",
			Recipients: []string{"core-649848208d3e@" + domain},
			Subject:    "Export does nothing",
			ReceivedAt: time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC),
		}, nil)
	h.intake.EXPECT().
		AddressOf(gomock.Any(), "core-649848208d3e", domain).
		Return(h.address(), nil)

	var recorded entity.IntakeDelivery

	h.intake.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, delivery entity.IntakeDelivery) (uuid.UUID, error) {
			recorded = delivery

			return h.deliveryID, nil
		})

	var queued entity.IntakeDeliveryPayload

	h.jobs.EXPECT().
		EnqueueIntakeDelivery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.IntakeDeliveryPayload) error {
			queued = payload

			return nil
		})

	deliveryID, err := h.service.Accept(context.Background(), http.Header{}, nil)
	if err != nil {
		t.Fatalf("accepting refused: %v", err)
	}

	if recorded.ExternalID != "inb_1" {
		t.Fatalf(
			"the delivery records %q as the provider's id, want inb_1. Redelivery is refused on "+
				"that id alone, so getting it wrong files the same report twice.",
			recorded.ExternalID,
		)
	}

	if recorded.Sender != "rae@northwind.co" {
		t.Fatalf("sender=%q, want the bare address", recorded.Sender)
	}

	if recorded.TeamID != h.teamID || recorded.WorkspaceID != h.workspaceID {
		t.Fatalf("the delivery was filed against the wrong team")
	}

	if queued.DeliveryID != deliveryID {
		t.Fatalf("queued %v, recorded %v", queued.DeliveryID, deliveryID)
	}
}

func TestAMessageBecomesAnIssueNobodyInTheWorkspaceIsCreditedWith(t *testing.T) {
	h := newHarness(t)

	h.intake.EXPECT().Delivery(gomock.Any(), h.deliveryID).Return(h.delivery(), nil)
	h.intake.EXPECT().Address(gomock.Any(), h.workspaceID, h.teamID).Return(h.address(), nil)
	h.mail.EXPECT().
		Fetch(gomock.Any(), "inb_1").
		Return(entity.InboundMessage{
			ExternalID: "inb_1",
			Sender:     "rae@northwind.co",
			Recipients: []string{"core-649848208d3e@" + domain},
			Subject:    "Export does nothing",
			Text:       "I press it and the page just sits there.",
		}, nil)

	var filed service.CreateIssueInput

	issueID := uuid.New()

	h.issues.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.CreateIssueInput) (entity.Issue, error) {
			filed = input

			return entity.Issue{ID: issueID}, nil
		})

	var settled struct {
		outcome entity.IntakeDeliveryOutcome
		issueID uuid.UUID
	}

	h.intake.EXPECT().
		Settle(gomock.Any(), h.deliveryID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			outcome entity.IntakeDeliveryOutcome,
			issue uuid.UUID,
			_ string,
			_ time.Time,
		) error {
			settled.outcome = outcome
			settled.issueID = issue

			return nil
		})

	if err := h.service.Apply(context.Background(), h.deliveryID); err != nil {
		t.Fatalf("filing refused: %v", err)
	}

	if filed.Source != entity.TriageSourceEmail {
		t.Fatalf(
			"the issue was filed as %q, want email. The source is what puts it in the Email queue "+
				"and what holds it for review however the team routes its members.",
			filed.Source,
		)
	}

	if filed.Title != "Export does nothing" {
		t.Fatalf("title=%q, want the subject", filed.Title)
	}

	if settled.outcome != entity.IntakeDeliveryFiled || settled.issueID != issueID {
		t.Fatalf(
			"the delivery settled as %q against %v, want filed against %v. An unsettled delivery "+
				"is retried, and a retry files the report a second time.",
			settled.outcome, settled.issueID, issueID,
		)
	}
}

func TestMailToAnAddressThatHasBeenReplacedIsNotFiled(t *testing.T) {
	h := newHarness(t)

	rotated := h.address()
	rotated.LocalPart = "core-1f0c74a91b52"

	h.intake.EXPECT().Delivery(gomock.Any(), h.deliveryID).Return(h.delivery(), nil)
	h.intake.EXPECT().Address(gomock.Any(), h.workspaceID, h.teamID).Return(rotated, nil)
	h.intake.EXPECT().
		Settle(gomock.Any(), h.deliveryID, entity.IntakeDeliveryIgnored, uuid.Nil, gomock.Any(), gomock.Any()).
		Return(nil)

	if err := h.service.Apply(context.Background(), h.deliveryID); err != nil {
		t.Fatalf("applying refused: %v", err)
	}
}

func TestADeliveryAlreadyDealtWithIsNotFiledAgain(t *testing.T) {
	h := newHarness(t)

	settled := h.delivery()
	at := time.Date(2026, 2, 12, 10, 5, 0, 0, time.UTC)
	settled.ProcessedAt = &at
	settled.Outcome = entity.IntakeDeliveryFiled

	h.intake.EXPECT().Delivery(gomock.Any(), h.deliveryID).Return(settled, nil)

	if err := h.service.Apply(context.Background(), h.deliveryID); err != nil {
		t.Fatalf("applying refused: %v", err)
	}
}
