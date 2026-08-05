package authorizer

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	agentthrottlerepo "github.com/usenorn/norn/internal/repository/agentthrottle"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	authpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
)

type agentHarness struct {
	authorizer  *casbinAuthorizer
	memberships *membershiprepo.MockMembership
	teams       *teamrepo.MockTeam
	throttle    *agentthrottlerepo.MockAgentThrottle

	agentAccount uuid.UUID
	owner        uuid.UUID
	agentID      uuid.UUID
	workspaceID  uuid.UUID
}

func newAgentHarness(t *testing.T, enforcer policyEnforcer) *agentHarness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &agentHarness{
		memberships:  membershiprepo.NewMockMembership(ctrl),
		teams:        teamrepo.NewMockTeam(ctrl),
		throttle:     agentthrottlerepo.NewMockAgentThrottle(ctrl),
		agentAccount: uuid.New(),
		owner:        uuid.New(),
		agentID:      uuid.New(),
		workspaceID:  uuid.New(),
	}

	authPolicies := authpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl)
	workspaces := workspacerepo.NewMockWorkspace(ctrl)
	accounts := accountrepo.NewMockAccount(ctrl)

	authPolicies.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
			return entity.WorkspaceAuthPolicy{
				WorkspaceID: workspaceID,
				Enforcement: entity.AuthEnforcementAny,
			}, nil
		}).
		AnyTimes()

	workspaces.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, workspaceID uuid.UUID) (entity.Workspace, error) {
			return entity.Workspace{ID: workspaceID, Status: entity.WorkspaceStatusActive}, nil
		}).
		AnyTimes()

	accounts.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Account{}, entity.ErrAccountNotFound).
		AnyTimes()

	h.throttle.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		Return(1, nil).
		AnyTimes()

	h.authorizer = &casbinAuthorizer{
		enforcer:      enforcer,
		memberships:   h.memberships,
		authPolicies:  authPolicies,
		workspaces:    workspaces,
		teams:         h.teams,
		accounts:      accounts,
		agentThrottle: h.throttle,
		audit:         silentAudit(ctrl),
	}

	return h
}

func (h *agentHarness) actor() entity.Actor {
	return entity.Actor{
		Kind:           entity.ActorKindAgent,
		AccountID:      h.agentAccount,
		AgentID:        &h.agentID,
		AgentAllowance: entity.AgentActionsPerWindow,
		OwnerAccountID: h.owner,
	}
}

func (h *agentHarness) decide(request entity.AccessRequest) (entity.Decision, error) {
	ctx := identity.WithActor(context.Background(), h.actor())

	return h.authorizer.Decide(ctx, request)
}

func TestAnAgentIsAuthorisedAgainstItsOwnersMembership(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	var asked uuid.UUID

	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, _ uuid.UUID, accountID uuid.UUID) (entity.Membership, error) {
			asked = accountID

			return entity.Membership{Role: entity.MembershipRoleAdmin}, nil
		})

	if _, err := h.decide(entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: h.workspaceID,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if asked != h.owner {
		t.Fatalf(
			"the membership looked up was %v, want the owner %v. An agent authorised against its "+
				"own membership could outlive every restriction placed on the person it acts for.",
			asked, h.owner,
		)
	}
}

func TestAnAgentLosesTheWorkspaceWhenItsOwnerDoes(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), h.owner).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	_, err := h.decide(entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: h.workspaceID,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonNotAMember {
		t.Fatalf("Decide error = %v, want a not_a_member denial once the owner has left", err)
	}
}

func TestAnAgentSeesOnlyTheTeamsItsOwnerCanSee(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	ownersTeam := uuid.New()

	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), h.owner).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)

	var scopedTo uuid.UUID

	h.teams.EXPECT().
		ListVisibleTo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ any, _ uuid.UUID, accountID uuid.UUID, _ entity.TeamStatus, _ bool,
		) ([]entity.Team, error) {
			scopedTo = accountID

			return []entity.Team{{ID: ownersTeam}}, nil
		})

	decision, err := h.decide(entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: h.workspaceID,
		Scoped:      true,
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if scopedTo != h.owner {
		t.Fatalf(
			"team visibility was resolved for %v, want the owner %v. Taking somebody off a team "+
				"has to take their agent off it in the same moment.",
			scopedTo, h.owner,
		)
	}

	if !decision.Scope.Covers(ownersTeam) {
		t.Error("the agent did not reach a team its owner can see")
	}

	if decision.Scope.Covers(uuid.New()) {
		t.Error("the agent reached a team its owner cannot see")
	}
}

func TestAnAgentPastItsAllowanceIsRefused(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	throttle := agentthrottlerepo.NewMockAgentThrottle(gomock.NewController(t))
	throttle.EXPECT().
		Record(gomock.Any(), h.agentID).
		Return(entity.AgentActionsPerWindow+1, nil)

	h.authorizer.agentThrottle = throttle

	_, err := h.decide(entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: h.workspaceID,
	})

	var denied entity.AccessDeniedError
	if !errors.As(err, &denied) || denied.Reason != entity.DenyReasonAgentRateLimited {
		t.Fatalf("Decide error = %v, want an agent_rate_limited denial", err)
	}

	if !errors.Is(err, entity.ErrAgentRateLimited) {
		t.Error("the denial did not surface as ErrAgentRateLimited, so no edge can answer 429")
	}
}

func TestReadingIsNeverCountedAgainstAnAgentsAllowance(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	throttle := agentthrottlerepo.NewMockAgentThrottle(gomock.NewController(t))
	h.authorizer.agentThrottle = throttle

	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), h.owner).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)

	if _, err := h.decide(entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionRead,
		WorkspaceID: h.workspaceID,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
}

func TestAPersonIsNeverPacedLikeAnAgent(t *testing.T) {
	h := newAgentHarness(t, &stubEnforcer{allow: true})

	throttle := agentthrottlerepo.NewMockAgentThrottle(gomock.NewController(t))
	h.authorizer.agentThrottle = throttle

	person := uuid.New()

	h.memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), person).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)

	ctx := actingAs(person, entity.SessionAuthMethodPassword)

	if _, err := h.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      entity.ActionManage,
		WorkspaceID: h.workspaceID,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
}
