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

	"github.com/usenorn/norn/internal/entity"
)

func TestResendReplacesTheStoredTokenSoThePriorLinkStopsResolving(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	priorHash := invitation.TokenHash

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)

	var storedHash []byte

	var storedExpiry time.Time

	h.invitations.EXPECT().
		Refresh(gomock.Any(), invitation.ID, gomock.Any(), gomock.Any(), entity.InvitationDeliveryPending).
		DoAndReturn(func(
			_ context.Context,
			_ uuid.UUID,
			tokenHash []byte,
			expiresAt time.Time,
			delivery entity.InvitationDelivery,
		) (entity.Invitation, error) {
			storedHash = tokenHash
			storedExpiry = expiresAt

			refreshed := invitation
			refreshed.TokenHash = tokenHash
			refreshed.ExpiresAt = expiresAt
			refreshed.Delivery = delivery

			return refreshed, nil
		})

	enqueued := h.captureEnqueued()

	issued, err := h.service.Resend(actingAs(actor), workspace.ID, invitation.ID)
	if err != nil {
		t.Fatalf("Resend: %v", err)
	}

	if bytes.Equal(storedHash, priorHash) {
		t.Fatal("resend kept the previous token hash, so the previous link still works")
	}

	token := strings.TrimPrefix(issued.URL, baseURL+"/accept-invitation?token=")

	if !bytes.Equal(storedHash, entity.HashInvitationToken(token)) {
		t.Fatal("the stored hash does not match the link handed back")
	}

	if bytes.Equal(entity.HashInvitationToken(token), priorHash) {
		t.Fatal("resend reissued the previous token")
	}

	if !storedExpiry.After(invitation.ExpiresAt) {
		t.Error("resend did not extend the expiry")
	}

	if len(*enqueued) != 1 || (*enqueued)[0].Token != token {
		t.Fatalf("resend enqueued %d mails, want one carrying the new token", len(*enqueued))
	}
}

func TestResendRefusesAnInvitationThatIsNoLongerPending(t *testing.T) {
	cases := []struct {
		name   string
		status entity.InvitationStatus
		want   error
	}{
		{name: "revoked", status: entity.InvitationStatusRevoked, want: entity.ErrInvitationRevoked},
		{name: "accepted", status: entity.InvitationStatusAccepted, want: entity.ErrInvitationAccepted},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			workspace := workspaceFixture()
			actor := uuid.New()
			invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
			invitation.Status = testCase.status

			h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
			h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)

			_, err := h.service.Resend(actingAs(actor), workspace.ID, invitation.ID)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("Resend error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestResendRefusesAnInvitationBelongingToAnotherWorkspace(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	invitation := pendingInvitation(uuid.New(), "ada@northwind.co", entity.MembershipRoleMember)

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)

	_, err := h.service.Resend(actingAs(actor), workspace.ID, invitation.ID)
	if !errors.Is(err, entity.ErrInvitationNotFound) {
		t.Fatalf("Resend error = %v, want ErrInvitationNotFound", err)
	}
}

func TestRevokedInvitationCannotBeAccepted(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)
	h.invitations.EXPECT().MarkRevoked(gomock.Any(), invitation.ID, gomock.Any()).Return(nil)

	if err := h.service.Revoke(actingAs(actor), workspace.ID, invitation.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	revoked := invitation
	revoked.Status = entity.InvitationStatusRevoked

	h.invitations.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashInvitationToken("a-stored-token")).
		Return(revoked, nil)

	_, err := h.service.Accept(context.Background(), acceptWithToken("a-stored-token"))
	if !errors.Is(err, entity.ErrInvitationRevoked) {
		t.Fatalf("Accept error = %v, want ErrInvitationRevoked", err)
	}
}

func TestRevokeRefusesAnAlreadyAcceptedInvitation(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	invitation.Status = entity.InvitationStatusAccepted

	h.expectAdminActor(workspace.ID, actor, entity.ActionManage)
	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)

	err := h.service.Revoke(actingAs(actor), workspace.ID, invitation.ID)
	if !errors.Is(err, entity.ErrInvitationAccepted) {
		t.Fatalf("Revoke error = %v, want ErrInvitationAccepted", err)
	}
}

func TestListRejectsAnUnknownStatusFilter(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()

	h.expectAdminActor(workspace.ID, actor, entity.ActionRead)

	_, err := h.service.List(actingAs(actor), workspace.ID, "expired")

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("List error = %v, want a ValidationError", err)
	}
}

func TestListPassesAnEmptyFilterThroughSoInvitationsShowBesideMembers(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	actor := uuid.New()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectAdminActor(workspace.ID, actor, entity.ActionRead)
	h.invitations.EXPECT().
		ListByWorkspaceID(gomock.Any(), workspace.ID, entity.InvitationStatus("")).
		Return([]entity.Invitation{invitation}, nil)

	listed, err := h.service.List(actingAs(actor), workspace.ID, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(listed) != 1 || listed[0].ID != invitation.ID {
		t.Fatalf("List returned %v, want the workspace invitation", listed)
	}
}
