package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	membershiprepo "github.com/usenorn/norn/internal/repository/membership"
	teamrepo "github.com/usenorn/norn/internal/repository/team"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	workflowstaterepo "github.com/usenorn/norn/internal/repository/workflowstate"
	"github.com/usenorn/norn/internal/service"
	agentsvc "github.com/usenorn/norn/internal/service/agent"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	issuesvc "github.com/usenorn/norn/internal/service/issue"
)

type harness struct {
	agents     *agentrepo.MockAgent
	accounts   *accountrepo.MockAccount
	members    *membershiprepo.MockMembership
	tokens     *apitokenrepo.MockAPIToken
	proposals  *agentproposalrepo.MockAgentProposal
	issues     *issuesvc.MockIssues
	authorizer *authorizersvc.MockAuthorizer
	service    service.Agents

	workspaceID uuid.UUID
	adminID     uuid.UUID
}

func newHarness(t *testing.T, role entity.MembershipRole) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	h := &harness{
		agents:      agentrepo.NewMockAgent(ctrl),
		accounts:    accountrepo.NewMockAccount(ctrl),
		members:     membershiprepo.NewMockMembership(ctrl),
		tokens:      apitokenrepo.NewMockAPIToken(ctrl),
		proposals:   agentproposalrepo.NewMockAgentProposal(ctrl),
		issues:      issuesvc.NewMockIssues(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: uuid.New(),
		adminID:     uuid.New(),
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.adminID},
				Role:  role,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.service = agentsvc.New(
		h.agents,
		agentsettingrepo.NewMockAgentSetting(ctrl),
		h.proposals,
		h.accounts,
		h.members,
		h.tokens,
		teamrepo.NewMockTeam(ctrl),
		activityrepo.NewMockActivity(ctrl),
		workflowstaterepo.NewMockWorkflowState(ctrl),
		h.issues,
		(service.IssueComments)(nil),
		(service.Checks)(nil),
		h.authorizer,
		transactor,
		silentAudit(ctrl),
	)

	return h
}

func readScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceIssue, entity.ActionRead)}
}

func adminOnlyScopes() entity.APIScopeSet {
	return entity.APIScopeSet{entity.NewAPIScope(entity.ResourceLabel, entity.ActionManage)}
}

func TestRegisteringAnAgentCreatesANonHumanAccountThatCannotSignIn(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	var created entity.Account

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			account.ID = uuid.New()
			created = account

			return account, nil
		})

	h.members.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)
	h.agents.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, agent entity.Agent) (entity.Agent, error) {
			agent.ID = uuid.New()

			return agent, nil
		})
	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			return token, nil
		})

	registered, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID: h.workspaceID,
		Name:        "triage-bot",
		Scopes:      readScopes(),
		AllTeams:    true,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if created.Kind != entity.AccountKindAgent {
		t.Fatalf("account kind = %q, want agent", created.Kind)
	}

	if created.PasswordHash != "" {
		t.Fatal("an agent was given a password, so it could sign in as a person")
	}

	if !strings.HasPrefix(registered.Value, entity.APITokenPrefix) {
		t.Error("the agent was not handed a usable credential")
	}
}

func TestAnAgentIsRegisteredAsAViewerBecauseItsMembershipConfersNothing(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)

	var membership entity.Membership

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			account.ID = uuid.New()

			return account, nil
		})
	h.members.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, created entity.Membership) (entity.Membership, error) {
			membership = created

			return created, nil
		})
	h.agents.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, agent entity.Agent) (entity.Agent, error) {
			return agent, nil
		})
	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			return token, nil
		})

	if _, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID: h.workspaceID,
		Name:        "triage-bot",
		Scopes:      readScopes(),
		AllTeams:    true,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if membership.Role != entity.MembershipRoleViewer {
		t.Fatalf(
			"the agent was registered as %q. Its membership exists so it can be assigned and "+
				"mentioned; every permission it has comes from its owner, so the row must confer "+
				"the least the column allows.",
			membership.Role,
		)
	}
}

func TestAnAgentCannotBeGivenMoreThanItsOwnerHas(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	owner := uuid.New()

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, owner).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)

	_, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID:    h.workspaceID,
		Name:           "triage-bot",
		OwnerAccountID: owner,
		Scopes:         adminOnlyScopes(),
		AllTeams:       true,
	})

	if !errors.Is(err, entity.ErrAPITokenScopeExceeds) {
		t.Fatalf(
			"Register error = %v, want ErrAPITokenScopeExceeds. An admin registering an agent for "+
				"somebody else must not hand it more than that person can do.",
			err,
		)
	}
}

func TestAnAgentCannotBeOwnedBySomebodyOutsideTheWorkspace(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	outsider := uuid.New()

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, outsider).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	_, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID:    h.workspaceID,
		Name:           "triage-bot",
		OwnerAccountID: outsider,
		Scopes:         readScopes(),
		AllTeams:       true,
	})

	if !errors.Is(err, entity.ErrAgentOwnerInvalid) {
		t.Fatalf("Register error = %v, want ErrAgentOwnerInvalid", err)
	}
}

func TestDisablingAnAgentAlsoKillsItsCredentials(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID, agentAccount := uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, AccountID: agentAccount, Status: entity.AgentStatusActive}, nil)

	h.agents.EXPECT().Disable(gomock.Any(), h.workspaceID, agentID, gomock.Any()).Return(nil)

	var revoked uuid.UUID

	h.tokens.EXPECT().
		RevokeAllByAccount(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, accountID uuid.UUID, _ any) error {
			revoked = accountID

			return nil
		})

	if err := h.service.Disable(context.Background(), h.workspaceID, agentID); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	if revoked != agentAccount {
		t.Fatal(
			"disabling an agent left its tokens live. The status check refuses it on the next " +
				"request, but a credential that still authenticates is one bug away from working.",
		)
	}
}

func TestAnApprovedProposalCannotBeApprovedTwice(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	proposalID := uuid.New()

	h.proposals.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, proposalID).
		Return(entity.AgentProposal{
			ID:          proposalID,
			WorkspaceID: h.workspaceID,
			Status:      entity.AgentProposalApplied,
		}, nil)

	_, err := h.service.Approve(context.Background(), h.workspaceID, proposalID)

	if !errors.Is(err, entity.ErrAgentProposalSettled) {
		t.Fatalf(
			"Approve error = %v, want ErrAgentProposalSettled. Approving twice would apply the "+
				"agent's change twice.",
			err,
		)
	}
}

func TestATokenMayNotRegisterOrApproveOnAnAgentsBehalf(t *testing.T) {
	ctrl := gomock.NewController(t)

	authorizer := authorizersvc.NewMockAuthorizer(ctrl)
	authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		Return(entity.Decision{
			Actor: entity.Actor{Kind: entity.ActorKindToken, AccountID: uuid.New()},
			Role:  entity.MembershipRoleAdmin,
		}, nil).
		AnyTimes()

	svc := agentsvc.New(
		agentrepo.NewMockAgent(ctrl),
		agentsettingrepo.NewMockAgentSetting(ctrl),
		agentproposalrepo.NewMockAgentProposal(ctrl),
		accountrepo.NewMockAccount(ctrl),
		membershiprepo.NewMockMembership(ctrl),
		apitokenrepo.NewMockAPIToken(ctrl),
		teamrepo.NewMockTeam(ctrl),
		activityrepo.NewMockActivity(ctrl),
		workflowstaterepo.NewMockWorkflowState(ctrl),
		(service.Issues)(nil),
		(service.IssueComments)(nil),
		(service.Checks)(nil),
		authorizer,
		transactorrepo.NewMockTransactor(ctrl),
		silentAudit(ctrl),
	)

	if _, err := svc.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID: uuid.New(),
		Name:        "triage-bot",
		Scopes:      readScopes(),
		AllTeams:    true,
	}); !errors.Is(err, entity.ErrAPITokenMintForbidden) {
		t.Fatalf("Register error = %v, want ErrAPITokenMintForbidden", err)
	}
}
