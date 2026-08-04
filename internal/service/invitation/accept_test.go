package invitation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	acceptToken   = "a-stored-token"
	acceptSession = "a-fresh-session-token"
)

func acceptWithToken(token string) service.AcceptInvitationInput {
	return service.AcceptInvitationInput{Token: token}
}

func (h *harness) expectInvitationByToken(invitation entity.Invitation) {
	h.invitations.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashInvitationToken(acceptToken)).
		Return(invitation, nil)
}

func TestAcceptCreatesTheAccountMembershipAndSessionForSomeoneBrandNew(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleAdmin)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.expectNoAccount("ada@northwind.co")

	var registered service.RegisterAccountInput

	h.registration.EXPECT().
		Register(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, input service.RegisterAccountInput) (entity.Account, error) {
			registered = input

			return account, nil
		})

	h.sessions.EXPECT().
		SignIn(gomock.Any(), gomock.Any()).
		Return(service.IssuedSession{
			Session: entity.Session{ID: uuid.New(), AccountID: account.ID},
			Token:   acceptSession,
		}, nil)

	h.invitations.EXPECT().
		MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).
		Return(nil)

	var membership entity.Membership

	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, created entity.Membership) (entity.Membership, error) {
			membership = created
			created.ID = uuid.New()

			return created, nil
		})

	accepted, err := h.service.Accept(context.Background(), service.AcceptInvitationInput{
		Token:       acceptToken,
		DisplayName: "Ada Lovelace",
		Timezone:    "Europe/London",
		Password:    "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if registered.Email != invitation.Email {
		t.Errorf("registered %q, want the invited address %q", registered.Email, invitation.Email)
	}

	if registered.DisplayName != "Ada Lovelace" {
		t.Errorf("registered display name = %q, want the submitted one", registered.DisplayName)
	}

	if membership.Role != entity.MembershipRoleAdmin {
		t.Errorf("granted role = %q, want the invited role admin", membership.Role)
	}

	if membership.WorkspaceID != workspace.ID {
		t.Errorf("granted membership in %s, want %s", membership.WorkspaceID, workspace.ID)
	}

	if !accepted.SignedIn || accepted.Session.Token != acceptSession {
		t.Error("a brand-new account must be signed in as it accepts")
	}

	if accepted.Workspace.Slug != workspace.Slug {
		t.Errorf("accepted workspace = %q, want %q", accepted.Workspace.Slug, workspace.Slug)
	}
}

func TestAcceptGrantsMembershipOnlyWhenAlreadySignedIn(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
	h.invitations.EXPECT().MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).Return(nil)
	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, created entity.Membership) (entity.Membership, error) {
			created.ID = uuid.New()

			return created, nil
		})

	accepted, err := h.service.Accept(actingAs(account.ID), acceptWithToken(acceptToken))
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if accepted.SignedIn {
		t.Error("an already signed-in account must not be issued a second session")
	}

	if accepted.Membership.AccountID != account.ID {
		t.Errorf("membership granted to %s, want the signed-in account", accepted.Membership.AccountID)
	}
}

func TestAcceptGrantsTheTeamMembershipsTheInvitationCarried(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	mobile := uuid.New()
	platform := uuid.New()
	invitation.TeamIDs = []uuid.UUID{mobile, platform}

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
	h.invitations.EXPECT().MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).Return(nil)
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)

	granted := h.captureGrantedTeams()

	if _, err := h.service.Accept(actingAs(account.ID), acceptWithToken(acceptToken)); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if len(*granted) != 2 || (*granted)[0].TeamID != mobile || (*granted)[1].TeamID != platform {
		t.Fatalf("granted teams = %+v, want both teams the invitation named", *granted)
	}

	if (*granted)[0].AccountID != account.ID || (*granted)[0].WorkspaceID != workspace.ID {
		t.Fatalf("grant = %+v, want it scoped to the accepting account and workspace", (*granted)[0])
	}
}

func TestAcceptFallsBackToTheWorkspaceDefaultTeamWhenTheInvitationNamesNone(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	defaultTeam := uuid.New()
	workspace.DefaultTeamID = &defaultTeam

	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
	h.invitations.EXPECT().MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).Return(nil)
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)

	granted := h.captureGrantedTeams()

	if _, err := h.service.Accept(actingAs(account.ID), acceptWithToken(acceptToken)); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if len(*granted) != 1 || (*granted)[0].TeamID != defaultTeam {
		t.Fatalf("granted teams = %+v, want the workspace default team", *granted)
	}
}

func TestAcceptPrefersTheInvitationTeamsOverTheWorkspaceDefault(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	defaultTeam := uuid.New()
	workspace.DefaultTeamID = &defaultTeam

	named := uuid.New()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	invitation.TeamIDs = []uuid.UUID{named}
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
	h.invitations.EXPECT().MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).Return(nil)
	h.memberships.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)

	granted := h.captureGrantedTeams()

	if _, err := h.service.Accept(actingAs(account.ID), acceptWithToken(acceptToken)); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	if len(*granted) != 1 || (*granted)[0].TeamID != named {
		t.Fatalf("granted teams = %+v, want only the team the invitation named", *granted)
	}
}

func TestAcceptRefusesASignedInAccountAtADifferentAddress(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	other := accountFixture("jun@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), other.ID).Return(other, nil)

	_, err := h.service.Accept(actingAs(other.ID), acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrInvitationAddressMismatch) {
		t.Fatalf("Accept error = %v, want ErrInvitationAddressMismatch", err)
	}
}

func TestAcceptTellsAnExistingAccountToSignInFirst(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByEmail(gomock.Any(), account.Email).Return(account, nil)

	_, err := h.service.Accept(context.Background(), acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrAccountEmailTaken) {
		t.Fatalf("Accept error = %v, want ErrAccountEmailTaken", err)
	}
}

func TestAcceptRefusesAnExpiredLink(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	invitation.ExpiresAt = time.Now().UTC().Add(-time.Minute)

	h.expectInvitationByToken(invitation)

	_, err := h.service.Accept(context.Background(), acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrInvitationExpired) {
		t.Fatalf("Accept error = %v, want ErrInvitationExpired", err)
	}
}

func TestAcceptRefusesALinkThatWasAlreadyUsed(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	invitation.Status = entity.InvitationStatusAccepted

	h.expectInvitationByToken(invitation)

	_, err := h.service.Accept(context.Background(), acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrInvitationAccepted) {
		t.Fatalf("Accept error = %v, want ErrInvitationAccepted", err)
	}
}

func TestAcceptRefusesAnEmptyToken(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Accept(context.Background(), acceptWithToken(""))
	if !errors.Is(err, entity.ErrInvitationTokenInvalid) {
		t.Fatalf("Accept error = %v, want ErrInvitationTokenInvalid", err)
	}
}

func TestAcceptRoutesThroughTheProviderWhereTheWorkspaceEnforcesSSO(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectInvitationByToken(invitation)
	h.expectJoiningRefused(workspace.ID, entity.ErrWorkspaceAuthMethodNotPermitted)

	_, err := h.service.Accept(context.Background(), acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf("Accept error = %v, want ErrWorkspaceAuthMethodNotPermitted", err)
	}
}

func TestAcceptRefusesAPasswordSessionWhereTheWorkspaceEnforcesSSO(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoiningRefused(workspace.ID, entity.ErrWorkspaceAuthMethodNotPermitted)

	ctx := actingAsSession(account.ID, entity.SessionAuthMethodPassword)

	_, err := h.service.Accept(ctx, acceptWithToken(acceptToken))
	if !errors.Is(err, entity.ErrWorkspaceAuthMethodNotPermitted) {
		t.Fatalf("Accept error = %v, want ErrWorkspaceAuthMethodNotPermitted", err)
	}
}

func TestAcceptAdmitsAnSSOSessionWhereTheWorkspaceEnforcesSSO(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)
	account := accountFixture("ada@northwind.co")

	h.expectInvitationByToken(invitation)
	h.expectJoining(workspace)
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
	h.invitations.EXPECT().MarkAccepted(gomock.Any(), invitation.ID, account.ID, gomock.Any()).Return(nil)
	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, created entity.Membership) (entity.Membership, error) {
			created.ID = uuid.New()

			return created, nil
		})

	ctx := actingAsSession(account.ID, entity.SessionAuthMethodSSO)

	if _, err := h.service.Accept(ctx, acceptWithToken(acceptToken)); err != nil {
		t.Fatalf("Accept: %v", err)
	}
}

func TestPreviewReportsWhetherTheInvitedAddressAlreadyHasAnAccount(t *testing.T) {
	cases := []struct {
		name   string
		lookup func(h *harness, email string)
		want   bool
	}{
		{
			name:   "no account yet",
			lookup: func(h *harness, email string) { h.expectNoAccount(email) },
			want:   false,
		},
		{
			name: "account already exists",
			lookup: func(h *harness, email string) {
				h.accounts.EXPECT().GetByEmail(gomock.Any(), email).Return(accountFixture(email), nil)
			},
			want: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			workspace := workspaceFixture()
			invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleAdmin)

			h.expectInvitationByToken(invitation)
			h.expectWorkspace(workspace)
			testCase.lookup(h, invitation.Email)
			h.expectAuthEnforcement(workspace.ID, entity.AuthEnforcementAny)

			preview, err := h.service.Preview(context.Background(), acceptToken)
			if err != nil {
				t.Fatalf("Preview: %v", err)
			}

			if preview.AccountExists != testCase.want {
				t.Errorf("AccountExists = %t, want %t", preview.AccountExists, testCase.want)
			}

			if preview.SSOEnforced {
				t.Error("SSOEnforced = true, want false for an unrestricted workspace")
			}

			if preview.Role != entity.MembershipRoleAdmin {
				t.Errorf("Role = %q, want the invited role", preview.Role)
			}

			if preview.Workspace.Name != workspace.Name {
				t.Errorf("Workspace.Name = %q, want %q", preview.Workspace.Name, workspace.Name)
			}
		})
	}
}

func TestPreviewSurfacesSSOEnforcementBeforeAnyoneSubmits(t *testing.T) {
	h := newHarness(t)
	workspace := workspaceFixture()
	invitation := pendingInvitation(workspace.ID, "ada@northwind.co", entity.MembershipRoleMember)

	h.expectInvitationByToken(invitation)
	h.expectWorkspace(workspace)
	h.expectNoAccount(invitation.Email)
	h.expectAuthEnforcement(workspace.ID, entity.AuthEnforcementSSO)

	preview, err := h.service.Preview(context.Background(), acceptToken)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if !preview.SSOEnforced {
		t.Error("SSOEnforced = false, want true so the screen routes through the provider")
	}
}

func TestPreviewRefusesAnUnrecognisedToken(t *testing.T) {
	h := newHarness(t)

	h.invitations.EXPECT().
		GetByTokenHash(gomock.Any(), entity.HashInvitationToken(acceptToken)).
		Return(entity.Invitation{}, entity.ErrInvitationNotFound)

	_, err := h.service.Preview(context.Background(), acceptToken)
	if !errors.Is(err, entity.ErrInvitationNotFound) {
		t.Fatalf("Preview error = %v, want ErrInvitationNotFound", err)
	}
}
