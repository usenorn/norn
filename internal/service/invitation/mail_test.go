package invitation_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func TestSendInvitationRecordsDeliveryAndLinksToTheAcceptScreen(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)
	h.expectWorkspace(workspace)

	var sent entity.Mail

	h.mailer.EXPECT().
		Send(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, mail entity.Mail) error {
			sent = mail

			return nil
		})

	h.invitations.EXPECT().
		SetDelivery(gomock.Any(), invitation.ID, entity.InvitationDeliverySent).
		Return(nil)

	if err := h.service.SendInvitation(context.Background(), invitation.ID, acceptToken); err != nil {
		t.Fatalf("SendInvitation: %v", err)
	}

	if sent.To != invitation.Email {
		t.Errorf("mail addressed to %q, want %q", sent.To, invitation.Email)
	}

	wantLink := baseURL + "/accept-invitation?token=" + acceptToken

	for _, body := range []string{sent.PlainBody, sent.HTMLBody} {
		if !strings.Contains(body, wantLink) {
			t.Errorf("mail body does not carry the accept link %q", wantLink)
		}

		if !strings.Contains(body, workspace.Name) {
			t.Errorf("mail body does not name the workspace %q", workspace.Name)
		}

		if !strings.Contains(body, "7 days") {
			t.Error("mail body does not state how long the link lasts")
		}
	}
}

func TestSendInvitationMarksDeliveryFailedAndReturnsTheErrorForRetry(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	bounced := errors.New("mailbox not found")

	h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)
	h.expectWorkspace(workspace)
	h.mailer.EXPECT().Send(gomock.Any(), gomock.Any()).Return(bounced)
	h.invitations.EXPECT().
		SetDelivery(gomock.Any(), invitation.ID, entity.InvitationDeliveryFailed).
		Return(nil)

	err := h.service.SendInvitation(context.Background(), invitation.ID, acceptToken)
	if !errors.Is(err, bounced) {
		t.Fatalf("SendInvitation error = %v, want the delivery failure so the job retries", err)
	}
}

func TestSendInvitationSkipsAnInvitationThatIsNoLongerUsable(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(invitation *entity.Invitation)
		mailing bool
	}{
		{
			name:   "revoked before the mail went out",
			mutate: func(i *entity.Invitation) { i.Status = entity.InvitationStatusRevoked },
		},
		{
			name:   "accepted before the mail went out",
			mutate: func(i *entity.Invitation) { i.Status = entity.InvitationStatusAccepted },
		},
		{
			name:   "expired before the mail went out",
			mutate: func(i *entity.Invitation) { i.ExpiresAt = time.Now().UTC().Add(-time.Minute) },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			invitation := pendingInvitation(workspaceFixture().ID, "ada@northwind.co", entity.MembershipRoleMember)
			testCase.mutate(&invitation)

			h.invitations.EXPECT().GetByID(gomock.Any(), invitation.ID).Return(invitation, nil)

			if err := h.service.SendInvitation(context.Background(), invitation.ID, acceptToken); err != nil {
				t.Fatalf("SendInvitation: %v", err)
			}
		})
	}
}

func TestSendInvitationIgnoresAnInvitationThatNoLongerExists(t *testing.T) {
	h := newHarness(t)
	invitation := pendingInvitation(workspaceFixture().ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.invitations.EXPECT().
		GetByID(gomock.Any(), invitation.ID).
		Return(entity.Invitation{}, entity.ErrInvitationNotFound)

	if err := h.service.SendInvitation(context.Background(), invitation.ID, acceptToken); err != nil {
		t.Fatalf("SendInvitation on a vanished invitation must not retry forever, got %v", err)
	}
}
