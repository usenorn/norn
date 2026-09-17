package invitation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
)

func (h *harness) expectPreviewOf(workspace entity.Workspace, invitation entity.Invitation) {
	h.expectInvitationByToken(invitation)
	h.expectWorkspace(workspace)
	h.expectNoAccount(invitation.Email)
	h.expectAuthEnforcement(workspace.ID, entity.AuthEnforcementAny)
}

func TestPreviewCarriesAShortLivedLinkToTheWorkspaceLogo(t *testing.T) {
	h := newHarness(t)

	workspace := workspaceFixture()
	workspace.LogoObjectKey = entity.WorkspaceLogoPrefix(workspace.ID) + "/logo.png"
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectPreviewOf(workspace, invitation)
	h.blobs.EXPECT().
		PresignGet(gomock.Any(), workspace.LogoObjectKey, entity.WorkspaceLogoServeSpec(workspace.LogoObjectKey), logoLinkTTL).
		Return("/v1/blobs/download/grant", nil)

	preview, err := h.service.Preview(context.Background(), acceptToken)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.LogoURL != "/v1/blobs/download/grant" {
		t.Errorf("logo link = %q, want the presigned link for the workspace logo", preview.LogoURL)
	}
}

func TestPreviewOfAWorkspaceWithoutALogoPresignsNothing(t *testing.T) {
	h := newHarness(t)

	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectPreviewOf(workspace, invitation)

	preview, err := h.service.Preview(context.Background(), acceptToken)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if preview.LogoURL != "" {
		t.Errorf("logo link = %q, want none", preview.LogoURL)
	}
}

func TestPreviewStillAnswersWhenTheLogoCannotBeLinked(t *testing.T) {
	h := newHarness(t)

	workspace := workspaceFixture()
	workspace.LogoObjectKey = entity.WorkspaceLogoPrefix(workspace.ID) + "/logo.png"
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectPreviewOf(workspace, invitation)
	h.blobs.EXPECT().
		PresignGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", errors.New("storage unreachable"))

	preview, err := h.service.Preview(context.Background(), acceptToken)
	if err != nil {
		t.Fatalf("Preview: %v, want the invitation to stay readable without its logo", err)
	}

	if preview.LogoURL != "" {
		t.Errorf("logo link = %q, want none so the page shows the letter", preview.LogoURL)
	}
}

func TestAnUnusableInvitationNeverLinksTheWorkspaceLogo(t *testing.T) {
	now := time.Now().UTC()

	cases := map[string]func(invitation *entity.Invitation){
		"revoked": func(invitation *entity.Invitation) {
			invitation.Status = entity.InvitationStatusRevoked
			invitation.RevokedAt = &now
		},
		"expired": func(invitation *entity.Invitation) {
			invitation.ExpiresAt = now.Add(-time.Minute)
		},
	}

	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)

			workspace := workspaceFixture()
			workspace.LogoObjectKey = entity.WorkspaceLogoPrefix(workspace.ID) + "/logo.png"
			invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
			spoil(&invitation)

			h.expectInvitationByToken(invitation)

			if _, err := h.service.Preview(context.Background(), acceptToken); err == nil {
				t.Fatal("Preview of an unusable invitation succeeded")
			}
		})
	}
}
