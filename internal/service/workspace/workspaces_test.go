package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	blobrepo "github.com/usenorn/norn/internal/repository/blob"
	breakglassrepo "github.com/usenorn/norn/internal/repository/breakglass"
	jobqueuerepo "github.com/usenorn/norn/internal/repository/jobqueue"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	ssoconnectionrepo "github.com/usenorn/norn/internal/repository/ssoconnection"
	ssoidentityrepo "github.com/usenorn/norn/internal/repository/ssoidentity"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	authpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	workspacesvc "github.com/usenorn/norn/internal/service/workspace"
)

const deletionGracePeriod = 720 * time.Hour

type harness struct {
	workspaces   *workspacerepo.MockWorkspace
	memberships  *membershiprepo.MockMembership
	accounts     *accountrepo.MockAccount
	teams        *teamrepo.MockTeam
	teamMembers  *teammemberrepo.MockTeamMember
	states       *workflowstaterepo.MockWorkflowState
	authPolicies *authpolicyrepo.MockWorkspaceAuthPolicy
	connections  *ssoconnectionrepo.MockSSOConnection
	identities   *ssoidentityrepo.MockSSOIdentity
	breakGlass   *breakglassrepo.MockBreakGlass
	producer     *jobqueuerepo.MockJobProducer
	blobs        *blobrepo.MockBlob
	authorizer   *authorizersvc.MockAuthorizer
	service      service.Workspaces
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h := &harness{
		workspaces:   workspacerepo.NewMockWorkspace(ctrl),
		memberships:  membershiprepo.NewMockMembership(ctrl),
		accounts:     accountrepo.NewMockAccount(ctrl),
		teams:        teamrepo.NewMockTeam(ctrl),
		teamMembers:  teammemberrepo.NewMockTeamMember(ctrl),
		states:       workflowstaterepo.NewMockWorkflowState(ctrl),
		authPolicies: authpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl),
		connections:  ssoconnectionrepo.NewMockSSOConnection(ctrl),
		identities:   ssoidentityrepo.NewMockSSOIdentity(ctrl),
		breakGlass:   breakglassrepo.NewMockBreakGlass(ctrl),
		producer:     jobqueuerepo.NewMockJobProducer(ctrl),
		blobs:        blobrepo.NewMockBlob(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.service = workspacesvc.New(
		h.workspaces,
		h.memberships,
		h.accounts,
		h.teams,
		h.teamMembers,
		h.states,
		h.authPolicies,
		h.connections,
		h.identities,
		h.breakGlass,
		h.producer,
		h.blobs,
		h.authorizer,
		transactor,
		config.Workspace{DeletionGracePeriod: deletionGracePeriod},
		silentAudit(ctrl),
	)

	return h
}

func (h *harness) expectActorMayManageMembers(workspaceID, actorID uuid.UUID) {
	h.expectActorMayManageMembersOn(workspaceID, actorID, entity.WorkspaceStatusActive)
}

func (h *harness) expectActorMayManageMembersOn(workspaceID, actorID uuid.UUID, status entity.WorkspaceStatus) {
	h.expectActorMayActOn(workspaceID, actorID, entity.ResourceMembership, entity.ActionManage, workspaceWithStatus(workspaceID, status))
}

func (h *harness) expectActorMayReadMembers(workspaceID, actorID uuid.UUID) {
	h.expectActorMayActOn(workspaceID, actorID, entity.ResourceMembership, entity.ActionRead, workspaceWithStatus(workspaceID, entity.WorkspaceStatusActive))
}

func workspaceWithStatus(workspaceID uuid.UUID, status entity.WorkspaceStatus) entity.Workspace {
	workspace := entity.Workspace{
		ID:       workspaceID,
		Slug:     "northwind",
		Name:     "Northwind",
		Status:   status,
		Timezone: entity.DefaultTimezone,
	}

	if status == entity.WorkspaceStatusPendingDeletion {
		requestedAt := time.Now().UTC()
		purgeAfter := requestedAt.Add(deletionGracePeriod)
		workspace.DeletionRequestedAt = &requestedAt
		workspace.PurgeAfter = &purgeAfter
	}

	return workspace
}

func (h *harness) expectAccount(account entity.Account) {
	h.accounts.EXPECT().GetByID(gomock.Any(), account.ID).Return(account, nil)
}

func (h *harness) expectMembership(
	workspaceID, accountID uuid.UUID,
	role entity.MembershipRole,
	source entity.MembershipSource,
) {
	h.memberships.EXPECT().
		Get(gomock.Any(), workspaceID, accountID).
		Return(entity.Membership{
			WorkspaceID: workspaceID,
			AccountID:   accountID,
			Role:        role,
			Source:      source,
		}, nil)
}

func passwordSession(accountID uuid.UUID) entity.Session {
	return entity.Session{
		ID:         uuid.New(),
		AccountID:  accountID,
		AuthMethod: entity.SessionAuthMethodPassword,
	}
}

func actingAs(accountID uuid.UUID) context.Context {
	return identity.WithSession(context.Background(), passwordSession(accountID))
}

func TestCreateMakesTheCreatingAccountAnAdministrator(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.workspaces.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, workspace entity.Workspace) (entity.Workspace, error) {
			workspace.ID = workspaceID

			return workspace, nil
		})

	var captured entity.Membership

	h.memberships.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, membership entity.Membership) (entity.Membership, error) {
			captured = membership

			return membership, nil
		})

	workspace, err := h.service.Create(actingAs(actorID), service.CreateWorkspaceInput{
		Slug: "acme-labs",
		Name: "Acme Labs",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if workspace.ID != workspaceID {
		t.Fatalf("workspace id = %v, want %v", workspace.ID, workspaceID)
	}

	if captured.AccountID != actorID || captured.Role != entity.MembershipRoleAdmin {
		t.Fatalf("creator membership = %+v, want the actor as admin", captured)
	}
}

func TestCreateRejectsAMalformedSlug(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Create(actingAs(uuid.New()), service.CreateWorkspaceInput{
		Slug: "Acme Labs",
		Name: "Acme Labs",
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create error = %v, want a ValidationError", err)
	}
}

func TestCreateWithoutAnIdentityIsRefused(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.Create(context.Background(), service.CreateWorkspaceInput{Slug: "acme-labs", Name: "Acme Labs"})
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("Create error = %v, want ErrAccountForbidden", err)
	}
}

func TestRemoveMemberIsRefusedForTheLastAdministrator(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, actorID, entity.MembershipRoleAdmin, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), actorID).
		Return([]uuid.UUID{workspaceID}, nil)

	err := h.service.RemoveMember(actingAs(actorID), workspaceID, actorID, nil)
	if !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatalf("RemoveMember error = %v, want ErrAccountLastWorkspaceAdmin", err)
	}
}

func TestRemoveMemberSucceedsForANonAdministrator(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), memberID).Return(nil, nil)
	h.memberships.EXPECT().Delete(gomock.Any(), workspaceID, memberID).Return(nil)

	if err := h.service.RemoveMember(actingAs(actorID), workspaceID, memberID, nil); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestDemotingTheLastAdministratorIsRefused(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	otherAdminID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, otherAdminID, entity.MembershipRoleAdmin, entity.MembershipSourceManual)
	h.workspaces.EXPECT().LockByIDs(gomock.Any(), []uuid.UUID{workspaceID}).Return(nil)
	h.memberships.EXPECT().
		ListWorkspaceIDsWithoutOtherActiveAdmin(gomock.Any(), otherAdminID).
		Return([]uuid.UUID{workspaceID}, nil)

	_, err := h.service.ChangeMemberRole(
		actingAs(actorID),
		workspaceID,
		otherAdminID,
		entity.MembershipRoleMember,
	)
	if !errors.Is(err, entity.ErrAccountLastWorkspaceAdmin) {
		t.Fatalf("ChangeMemberRole error = %v, want ErrAccountLastWorkspaceAdmin", err)
	}
}

func TestPromotingToAdministratorSkipsTheLastAdminGuard(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.expectMembership(workspaceID, memberID, entity.MembershipRoleMember, entity.MembershipSourceManual)
	h.memberships.EXPECT().
		UpdateRole(gomock.Any(), workspaceID, memberID, entity.MembershipRoleAdmin).
		Return(entity.Membership{WorkspaceID: workspaceID, AccountID: memberID, Role: entity.MembershipRoleAdmin}, nil)
	h.expectAccount(entity.Account{ID: memberID, DisplayName: "Rae Okafor", Email: "rae@northwind.co"})

	if _, err := h.service.ChangeMemberRole(
		actingAs(actorID),
		workspaceID,
		memberID,
		entity.MembershipRoleAdmin,
	); err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
}

func TestAddMemberIsRefusedForANonMemberActor(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	workspaceID := uuid.New()

	h.expectDecisionRefused(
		workspaceID,
		entity.ResourceMembership,
		entity.ActionManage,
		entity.AccessDeniedError{Reason: entity.DenyReasonNotAMember, Resource: entity.ResourceMembership},
	)

	_, err := h.service.AddMember(
		actingAs(actorID),
		workspaceID,
		uuid.New(),
		entity.MembershipRoleMember,
	)
	if !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("AddMember error = %v, want ErrAccountForbidden", err)
	}
}

func TestAddMemberIsRefusedForADeactivatedAccount(t *testing.T) {
	h := newHarness(t)

	actorID := uuid.New()
	memberID := uuid.New()
	workspaceID := uuid.New()

	h.expectActorMayManageMembers(workspaceID, actorID)
	h.accounts.EXPECT().GetByID(gomock.Any(), memberID).Return(entity.Account{
		ID:     memberID,
		Status: entity.AccountStatusDeactivated,
	}, nil)

	_, err := h.service.AddMember(
		actingAs(actorID),
		workspaceID,
		memberID,
		entity.MembershipRoleMember,
	)
	if !errors.Is(err, entity.ErrAccountDeactivated) {
		t.Fatalf("AddMember error = %v, want ErrAccountDeactivated", err)
	}
}

func TestOneAccountHoldsMembershipInSeveralWorkspaces(t *testing.T) {
	h := newHarness(t)

	accountID := uuid.New()

	held := []entity.Workspace{
		{ID: uuid.New(), Slug: "acme-labs", Name: "Acme Labs"},
		{ID: uuid.New(), Slug: "beta-works", Name: "Beta Works"},
	}

	h.workspaces.EXPECT().ListByAccountID(gomock.Any(), accountID).Return(held, nil)

	workspaces, err := h.service.ListForAccount(actingAs(accountID), accountID)
	if err != nil {
		t.Fatalf("ListForAccount: %v", err)
	}

	if len(workspaces) != len(held) {
		t.Fatalf("workspace count = %d, want %d", len(workspaces), len(held))
	}
}
