package team_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	notificationeventrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	teammemberrepo "github.com/usenorn/norn/internal/repository/teammember"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	authpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	teamsvc "github.com/usenorn/norn/internal/service/team"
)

type harness struct {
	teams        *teamrepo.MockTeam
	teamMembers  *teammemberrepo.MockTeamMember
	workspaces   *workspacerepo.MockWorkspace
	memberships  *membershiprepo.MockMembership
	accounts     *accountrepo.MockAccount
	authPolicies *authpolicyrepo.MockWorkspaceAuthPolicy
	states       *workflowstaterepo.MockWorkflowState
	notify       *notificationeventrepo.MockNotificationEvent
	transactor   *transactorrepo.MockTransactor
	authorizer   *authorizersvc.MockAuthorizer
	service      service.Teams
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		teams:        teamrepo.NewMockTeam(ctrl),
		teamMembers:  teammemberrepo.NewMockTeamMember(ctrl),
		workspaces:   workspacerepo.NewMockWorkspace(ctrl),
		memberships:  membershiprepo.NewMockMembership(ctrl),
		accounts:     accountrepo.NewMockAccount(ctrl),
		authPolicies: authpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl),
		states:       workflowstaterepo.NewMockWorkflowState(ctrl),
		notify:       notificationeventrepo.NewMockNotificationEvent(ctrl),
		transactor:   transactorrepo.NewMockTransactor(ctrl),
		authorizer:   authorizersvc.NewMockAuthorizer(ctrl),
	}

	h.transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.service = teamsvc.New(
		h.teams,
		h.teamMembers,
		h.workspaces,
		h.memberships,
		h.accounts,
		h.authPolicies,
		h.states,
		h.notify,
		h.authorizer,
		h.transactor,
		silentAudit(ctrl),
	)

	h.notify.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	return h
}

func (h *harness) expectActorMayAct(workspaceID, actorID uuid.UUID, role entity.MembershipRole, resource entity.Resource, action entity.Action) {
	h.expectActorMayActOn(workspaceID, actorID, role, resource, action, entity.WorkspaceStatusActive)
}

func (h *harness) expectActorMayActOn(
	workspaceID, actorID uuid.UUID,
	role entity.MembershipRole,
	resource entity.Resource,
	action entity.Action,
	status entity.WorkspaceStatus,
) {
	h.expectDecision(workspaceID, actorID, role, resource, action, status, entity.TeamScope{
		WorkspaceID:    workspaceID,
		AllTeams:       true,
		IncludePrivate: true,
	})
}

func (h *harness) expectActorSees(
	workspaceID, actorID uuid.UUID,
	role entity.MembershipRole,
	resource entity.Resource,
	action entity.Action,
	teamIDs ...uuid.UUID,
) {
	h.expectDecision(workspaceID, actorID, role, resource, action, entity.WorkspaceStatusActive, entity.TeamScope{
		WorkspaceID: workspaceID,
		TeamIDs:     teamIDs,
	})
}

func (h *harness) expectDecision(
	workspaceID, actorID uuid.UUID,
	role entity.MembershipRole,
	resource entity.Resource,
	action entity.Action,
	status entity.WorkspaceStatus,
	scope entity.TeamScope,
) {
	workspace := entity.Workspace{
		ID:       workspaceID,
		Slug:     "northwind",
		Name:     "Northwind",
		Status:   status,
		Timezone: entity.DefaultTimezone,
	}

	if status == entity.WorkspaceStatusPendingDeletion {
		requestedAt := time.Now().UTC()
		purgeAfter := requestedAt.Add(720 * time.Hour)
		workspace.DeletionRequestedAt = &requestedAt
		workspace.PurgeAfter = &purgeAfter
	}

	if workspace.Deleted() && action.RequiresLiveWorkspace() {
		h.authorizer.EXPECT().
			Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
			Return(entity.Decision{}, entity.AccessDeniedError{
				Reason:      entity.DenyReasonWorkspaceDeleted,
				Resource:    resource,
				Action:      action,
				WorkspaceID: workspaceID,
				PurgeAfter:  workspace.PurgeAfter,
			})

		return
	}

	h.authorizer.EXPECT().
		Decide(gomock.Any(), matchRequest(workspaceID, resource, action)).
		Return(entity.Decision{
			Actor:     entity.Actor{Kind: entity.ActorKindUser, AccountID: actorID},
			Role:      role,
			Workspace: workspace,
			Scope:     scope,
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

func (h *harness) expectTeam(team entity.Team) {
	h.teams.EXPECT().GetByID(gomock.Any(), team.ID).Return(team, nil)
}

func publicTeam(workspaceID, teamID uuid.UUID) entity.Team {
	return entity.Team{
		ID:          teamID,
		WorkspaceID: workspaceID,
		Key:         "MOB",
		Name:        "Mobile",
		Status:      entity.TeamStatusActive,
		Visibility:  entity.TeamVisibilityPublic,
	}
}

func privateTeam(workspaceID, teamID uuid.UUID) entity.Team {
	team := publicTeam(workspaceID, teamID)
	team.Key = "PLT"
	team.Name = "Data Platform"
	team.Visibility = entity.TeamVisibilityPrivate

	return team
}

func archivedTeam(workspaceID, teamID uuid.UUID) entity.Team {
	team := publicTeam(workspaceID, teamID)
	archivedAt := time.Now().UTC()
	team.Status = entity.TeamStatusArchived
	team.ArchivedAt = &archivedAt

	return team
}

func actingAs(accountID uuid.UUID) context.Context {
	return identity.WithSession(context.Background(), entity.Session{
		ID:         uuid.New(),
		AccountID:  accountID,
		AuthMethod: entity.SessionAuthMethodPassword,
	})
}

func (h *harness) expectStatesSeeded() {
	h.states.EXPECT().
		CreateMany(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, states []entity.WorkflowState) ([]entity.WorkflowState, error) {
			return states, nil
		})
}
