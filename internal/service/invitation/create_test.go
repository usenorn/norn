package invitation_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func TestCreateReportsOneOutcomePerAddressInTheOrderRequested(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	existing := accountFixture("jun@northwind.co")

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectAccountWithMembership(workspace.ID, existing)
	h.expectNoAccount("ada@northwind.co")
	h.captureCreatedInvitations()
	h.captureEnqueued()

	results, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("jun@northwind.co", "milo@@northwind.co", "ada@northwind.co"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := []entity.InvitationOutcome{
		entity.InvitationOutcomeAlreadyMember,
		entity.InvitationOutcomeInvalidEmail,
		entity.InvitationOutcomeCreated,
	}

	if len(results) != len(want) {
		t.Fatalf("got %d results, want %d", len(results), len(want))
	}

	for i, outcome := range want {
		if results[i].Outcome != outcome {
			t.Errorf("results[%d].Outcome = %q, want %q", i, results[i].Outcome, outcome)
		}
	}

	if results[1].URL != "" {
		t.Error("a malformed address must not produce an invitation link")
	}

	if results[0].URL != "" {
		t.Error("an existing member must not produce an invitation link")
	}
}

func TestCreateIssuesADistinctLinkPerInvitationAndStoresOnlyItsHash(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("jun@northwind.co")
	h.expectNoAccount("ada@northwind.co")
	created := h.captureCreatedInvitations()
	h.captureEnqueued()

	results, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("jun@northwind.co", "ada@northwind.co"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if results[0].URL == results[1].URL {
		t.Fatal("two invitations share the same link")
	}

	for i, result := range results {
		if !strings.HasPrefix(result.URL, baseURL+"/accept-invitation?token=") {
			t.Fatalf("results[%d].URL = %q, want an accept-invitation link", i, result.URL)
		}

		token := strings.TrimPrefix(result.URL, baseURL+"/accept-invitation?token=")

		if !bytes.Equal((*created)[i].TokenHash, entity.HashInvitationToken(token)) {
			t.Errorf("results[%d] stored a hash that does not match its issued token", i)
		}

		if bytes.Contains((*created)[i].TokenHash, []byte(token)) {
			t.Errorf("results[%d] stored the plaintext token", i)
		}
	}
}

func TestCreateRecordsTheRequestedRoleAndTheInvitingAccount(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("ada@northwind.co")
	created := h.captureCreatedInvitations()
	h.captureEnqueued()

	if _, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients: []service.InvitationRecipient{
			{Email: "ada@northwind.co", Role: entity.MembershipRoleAdmin},
		},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	invitation := (*created)[0]

	if invitation.Role != entity.MembershipRoleAdmin {
		t.Errorf("stored role = %q, want admin", invitation.Role)
	}

	if invitation.InvitedByAccountID == nil || *invitation.InvitedByAccountID != actor {
		t.Error("the invitation does not record who sent it")
	}

	if invitation.Status != entity.InvitationStatusPending {
		t.Errorf("stored status = %q, want pending", invitation.Status)
	}
}

func TestCreateNormalizesTheAddressBeforeStoringIt(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("ada@northwind.co")
	created := h.captureCreatedInvitations()
	h.captureEnqueued()

	results, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("  Ada@Northwind.CO "),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if (*created)[0].Email != "ada@northwind.co" {
		t.Errorf("stored email = %q, want %q", (*created)[0].Email, "ada@northwind.co")
	}

	if results[0].Email != "ada@northwind.co" {
		t.Errorf("reported email = %q, want the normalized address", results[0].Email)
	}
}

func TestCreateSupersedesAPendingInvitationForTheSameAddress(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("ada@northwind.co")
	h.captureEnqueued()

	revoked := 0

	h.invitations.EXPECT().
		RevokePendingByEmail(gomock.Any(), workspace.ID, "ada@northwind.co", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ uuid.UUID, _ string, _ time.Time) error {
			revoked++

			return nil
		})

	h.invitations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, invitation entity.Invitation) (entity.Invitation, error) {
			if revoked != 1 {
				t.Error("the previous pending invitation was not revoked before the replacement was written")
			}

			invitation.ID = uuid.New()

			return invitation, nil
		})

	if _, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("ada@northwind.co"),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestCreateWithoutMailConfiguredStillYieldsAUsableLink(t *testing.T) {
	h := newHarnessWithSMTP(t, config.SMTP{})
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("ada@northwind.co")
	created := h.captureCreatedInvitations()

	results, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("ada@northwind.co"),
	})
	if err != nil {
		t.Fatalf("Create with no mail configured must succeed, got %v", err)
	}

	if results[0].Outcome != entity.InvitationOutcomeCreated {
		t.Fatalf("outcome = %q, want created", results[0].Outcome)
	}

	if results[0].URL == "" {
		t.Fatal("an instance without mail must still hand back a link to distribute")
	}

	if (*created)[0].Delivery != entity.InvitationDeliveryLinkOnly {
		t.Errorf("delivery = %q, want link_only", (*created)[0].Delivery)
	}
}

func TestCreateEnqueuesExactlyOneMailPerCreatedInvitation(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.expectNoAccount("jun@northwind.co")
	h.expectNoAccount("ada@northwind.co")
	created := h.captureCreatedInvitations()
	enqueued := h.captureEnqueued()

	if _, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("jun@northwind.co", "milo@@northwind.co", "ada@northwind.co"),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(*enqueued) != 2 {
		t.Fatalf("enqueued %d mails, want one per created invitation (2)", len(*enqueued))
	}

	for i, payload := range *enqueued {
		if payload.InvitationID != (*created)[i].ID {
			t.Errorf("enqueued[%d] targets %s, want %s", i, payload.InvitationID, (*created)[i].ID)
		}

		if payload.Token == "" {
			t.Errorf("enqueued[%d] carries no token, so no link can be sent", i)
		}
	}

	if (*created)[0].Delivery != entity.InvitationDeliveryPending {
		t.Errorf("delivery = %q, want pending", (*created)[0].Delivery)
	}
}

func TestCreateRejectsAnUnknownRoleAsValidation(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)

	_, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients: []service.InvitationRecipient{
			{Email: "ada@northwind.co", Role: entity.MembershipRoleMember},
			{Email: "jun@northwind.co", Role: "owner"},
		},
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}

	if len(validation.Fields) != 1 || validation.Fields[0].Field != "invitations.1.role" {
		t.Fatalf("validation fields = %v, want the offending index named", validation.Fields)
	}
}

func TestCreateRefusesANonMember(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectDecisionRefused(
		workspace.ID,
		entity.ResourceInvitation,
		entity.ActionManage,
		entity.AccessDeniedError{Reason: entity.DenyReasonNotAMember, Resource: entity.ResourceInvitation},
	)

	_, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients:  recipients("ada@northwind.co"),
	})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("Create error = %v, want ErrAccountForbidden", err)
	}
}

func TestCreateRecordsTheTeamsTheInvitationGrants(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	teamID := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: workspace.ID,
		Status:      entity.TeamStatusActive,
	}, nil)
	h.expectNoAccount("ada@northwind.co")

	created := h.captureCreatedInvitations()
	h.captureEnqueued()

	if _, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients: []service.InvitationRecipient{{
			Email:   "ada@northwind.co",
			Role:    entity.MembershipRoleMember,
			TeamIDs: []uuid.UUID{teamID},
		}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(*created) != 1 || len((*created)[0].TeamIDs) != 1 || (*created)[0].TeamIDs[0] != teamID {
		t.Fatalf("stored invitation = %+v, want it carrying the requested team", *created)
	}
}

func TestCreateRejectsATeamFromAnotherWorkspace(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	teamID := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: uuid.New(),
		Status:      entity.TeamStatusActive,
	}, nil)

	_, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients: []service.InvitationRecipient{{
			Email:   "ada@northwind.co",
			Role:    entity.MembershipRoleMember,
			TeamIDs: []uuid.UUID{teamID},
		}},
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}

	if validation.Fields[0].Field != "invitations.0.teamIds" {
		t.Fatalf("field = %q, want the error attributed to the recipient's teamIds", validation.Fields[0].Field)
	}
}

func TestCreateRejectsAnArchivedTeam(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	teamID := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.teams.EXPECT().GetByID(gomock.Any(), teamID).Return(entity.Team{
		ID:          teamID,
		WorkspaceID: workspace.ID,
		Status:      entity.TeamStatusArchived,
	}, nil)

	_, err := h.service.Create(actingAs(actor), service.CreateInvitationsInput{
		WorkspaceID: workspace.ID,
		Recipients: []service.InvitationRecipient{{
			Email:   "ada@northwind.co",
			Role:    entity.MembershipRoleMember,
			TeamIDs: []uuid.UUID{teamID},
		}},
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}
}
