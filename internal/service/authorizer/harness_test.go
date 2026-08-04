package authorizer

import (
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	workspacerepo "github.com/usenorn/norn/internal/repository/workspace"
	authpolicyrepo "github.com/usenorn/norn/internal/repository/workspaceauthpolicy"
)

func newDecider(
	t *testing.T,
	enforcer policyEnforcer,
	role entity.MembershipRole,
	enforcement entity.AuthEnforcement,
) *casbinAuthorizer {
	t.Helper()

	authorizer, memberships := decider(t, enforcer, enforcement)

	memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Membership{Role: role}, nil).
		AnyTimes()

	return authorizer
}

func newDeciderWithMembershipError(t *testing.T, enforcer policyEnforcer, err error) *casbinAuthorizer {
	t.Helper()

	authorizer, memberships := decider(t, enforcer, entity.AuthEnforcementAny)

	memberships.EXPECT().
		Get(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.Membership{}, err).
		AnyTimes()

	return authorizer
}

func decider(
	t *testing.T,
	enforcer policyEnforcer,
	enforcement entity.AuthEnforcement,
) (*casbinAuthorizer, *membershiprepo.MockMembership) {
	t.Helper()

	ctrl := gomock.NewController(t)

	memberships := membershiprepo.NewMockMembership(ctrl)
	authPolicies := authpolicyrepo.NewMockWorkspaceAuthPolicy(ctrl)
	workspaces := workspacerepo.NewMockWorkspace(ctrl)
	teams := teamrepo.NewMockTeam(ctrl)
	accounts := accountrepo.NewMockAccount(ctrl)

	authPolicies.EXPECT().
		Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
			return entity.WorkspaceAuthPolicy{WorkspaceID: workspaceID, Enforcement: enforcement}, nil
		}).
		AnyTimes()

	workspaces.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, workspaceID uuid.UUID) (entity.Workspace, error) {
			return entity.Workspace{ID: workspaceID, Status: entity.WorkspaceStatusActive}, nil
		}).
		AnyTimes()

	teams.EXPECT().
		ListVisibleTo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	accounts.EXPECT().
		GetByID(gomock.Any(), gomock.Any()).
		Return(entity.Account{}, entity.ErrAccountNotFound).
		AnyTimes()

	return &casbinAuthorizer{
		enforcer:     enforcer,
		memberships:  memberships,
		authPolicies: authPolicies,
		workspaces:   workspaces,
		teams:        teams,
		accounts:     accounts,
	}, memberships
}
