package invitation_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	invitationrepo "github.com/usenorn/norn/internal/repository/invitation"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	mailerrepo "github.com/usenorn/norn/internal/repository/mailer"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	workspaceauthpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	"github.com/usenorn/norn/internal/service"
	accountsvc "github.com/usenorn/norn/internal/service/account"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	invitationsvc "github.com/usenorn/norn/internal/service/invitation"
	sessionsvc "github.com/usenorn/norn/internal/service/session"
)

const baseURL = "https://norn.test"

var configuredSMTP = config.SMTP{Host: "smtp.test", FromAddress: "no-reply@norn.test"}

type harness struct {
	invitations  *invitationrepo.MockInvitation
	memberships  *membershiprepo.MockMembership
	workspaces   *workspacerepo.MockWorkspace
	accounts     *accountrepo.MockAccount
	teams        *teamrepo.MockTeam
	teamMembers  *teammemberrepo.MockTeamMember
	authPolicies *workspaceauthpolicyrepo.MockWorkspaceAuthPolicy
	producer     *jobqueuerepo.MockJobProducer
	mailer       *mailerrepo.MockMailer
	transactor   *transactorrepo.MockTransactor
	authorizer   *authorizersvc.MockAuthorizer
	registration *accountsvc.MockAccounts
	sessions     *sessionsvc.MockSessions
	service      service.Invitations
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	return newHarnessWithSMTP(t, configuredSMTP)
}

func newHarnessWithSMTP(t *testing.T, smtp config.SMTP) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		invitations:  invitationrepo.NewMockInvitation(ctrl),
		memberships:  membershiprepo.NewMockMembership(ctrl),
		workspaces:   workspacerepo.NewMockWorkspace(ctrl),
		accounts:     accountrepo.NewMockAccount(ctrl),
		teams:        teamrepo.NewMockTeam(ctrl),
		teamMembers:  teammemberrepo.NewMockTeamMember(ctrl),
		authPolicies: workspaceauthpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl),
		producer:     jobqueuerepo.NewMockJobProducer(ctrl),
		mailer:       mailerrepo.NewMockMailer(ctrl),
		transactor:   transactorrepo.NewMockTransactor(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
		registration: accountsvc.NewMockAccounts(ctrl),
		sessions:     sessionsvc.NewMockSessions(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = invitationsvc.New(
		h.invitations,
		h.memberships,
		h.workspaces,
		h.accounts,
		h.teams,
		h.teamMembers,
		h.authPolicies,
		h.producer,
		h.mailer,
		h.transactor,
		h.authorizer,
		h.registration,
		h.sessions,
		config.App{BaseURL: baseURL},
		smtp,
		silentAudit(ctrl),
	)

	return h
}

func (h *harness) expectAdminActor(workspaceID, actorID uuid.UUID, action entity.Action) {
	h.expectDecision(workspaceID, actorID, entity.ResourceInvitation, action)
}

func (h *harness) expectDecision(workspaceID, actorID uuid.UUID, resource entity.Resource, action entity.Action) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: actorID},
			Role:  entity.MembershipRoleAdmin,
			Workspace: entity.Workspace{
				ID:       workspaceID,
				Slug:     "northwind",
				Name:     "Northwind",
				Status:   entity.WorkspaceStatusActive,
				Timezone: entity.DefaultTimezone,
			},
			Scope: entity.TeamScope{WorkspaceID: workspaceID, AllTeams: true},
		}, nil)
}

func (h *harness) expectDecisionRefused(workspaceID uuid.UUID, resource entity.Resource, action entity.Action, err error) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
		Return(entity.Decision{}, err)
}

func matchRequest(workspaceID uuid.UUID, resource entity.Resource, action entity.Action) gomock.Matcher {
	return gomock.Cond(func(request entity.AccessRequest) bool {
		return request.WorkspaceID == workspaceID &&
			request.Resource == resource &&
			request.Action == action
	})
}

func (h *harness) expectAuthEnforcement(workspaceID uuid.UUID, enforcement entity.AuthEnforcement) {
	h.authPolicies.EXPECT().
		Get(gomock.Any(), workspaceID).
		Return(entity.WorkspaceAuthPolicy{WorkspaceID: workspaceID, Enforcement: enforcement}, nil)
}

func (h *harness) expectWorkspace(workspace entity.Workspace) {
	h.workspaces.EXPECT().GetByID(gomock.Any(), workspace.ID).Return(workspace, nil)
}

func (h *harness) expectJoining(workspace entity.Workspace) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Cond(func(request entity.AccessRequest) bool {
			return request.WorkspaceID == workspace.ID && request.Joining
		})).
		Return(entity.Decision{Workspace: workspace}, nil)
}

func (h *harness) expectJoiningRefused(workspaceID uuid.UUID, err error) {
	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Cond(func(request entity.AccessRequest) bool {
			return request.WorkspaceID == workspaceID && request.Joining
		})).
		Return(entity.Decision{}, err)
}

func (h *harness) expectNoAccount(email string) {
	h.accounts.EXPECT().GetByEmail(gomock.Any(), email).Return(entity.Account{}, entity.ErrAccountNotFound)
}

func (h *harness) expectAccountWithMembership(workspaceID uuid.UUID, account entity.Account) {
	h.accounts.EXPECT().GetByEmail(gomock.Any(), account.Email).Return(account, nil)
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, account.ID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   account.ID,
			Role:        entity.MembershipRoleMember,
		}, nil)
}

func (h *harness) captureCreatedInvitations() *[]entity.Invitation {
	created := &[]entity.Invitation{}

	h.invitations.EXPECT().
		RevokePendingByEmail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	h.invitations.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, invitation entity.Invitation) (entity.Invitation, error) {
			invitation.ID = uuid.New()
			*created = append(*created, invitation)

			return invitation, nil
		}).
		AnyTimes()

	return created
}

func (h *harness) captureGrantedTeams() *[]entity.TeamMembership {
	granted := &[]entity.TeamMembership{}

	h.teamMembers.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.TeamMembership) (entity.TeamMembership, error) {
			*granted = append(*granted, membership)

			return membership, nil
		}).
		AnyTimes()

	return granted
}

func (h *harness) captureEnqueued() *[]entity.InvitationPayload {
	enqueued := &[]entity.InvitationPayload{}

	h.producer.EXPECT().
		EnqueueInvitation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload entity.InvitationPayload) error {
			*enqueued = append(*enqueued, payload)

			return nil
		}).
		AnyTimes()

	return enqueued
}

func actingAs(accountID uuid.UUID) context.Context {
	return identity.Into(context.Background(), accountID)
}

func actingAsSession(accountID uuid.UUID, method entity.SessionAuthMethod) context.Context {
	return identity.WithSession(context.Background(), entity.Session{
		ID:         uuid.New(),
		AccountID:  accountID,
		AuthMethod: method,
	})
}

func workspaceFixture() entity.Workspace {
	return entity.Workspace{
		ID:   uuid.New(),
		Slug: "northwind",
		Name: "Northwind",
	}
}

func accountFixture(email string) entity.Account {
	return entity.Account{
		ID:          uuid.New(),
		Status:      entity.AccountStatusActive,
		Email:       email,
		DisplayName: "Rae Okafor",
		Timezone:    "Europe/London",
		CreatedAt:   time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
}

func pendingInvitation(workspaceID uuid.UUID, email string, role entity.MembershipRole) entity.Invitation {
	now := time.Now().UTC()

	return entity.Invitation{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Email:       email,
		Role:        role,
		Status:      entity.InvitationStatusPending,
		Delivery:    entity.InvitationDeliveryPending,
		TokenHash:   entity.HashInvitationToken("a-stored-token"),
		InvitedAt:   now,
		ExpiresAt:   now.Add(entity.InvitationTokenTTL),
	}
}

func recipients(pairs ...string) []service.InvitationRecipient {
	list := make([]service.InvitationRecipient, len(pairs))
	for i, email := range pairs {
		list[i] = service.InvitationRecipient{Email: email, Role: entity.MembershipRoleMember}
	}

	return list
}
