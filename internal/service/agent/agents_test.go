package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	accountrepo "github.com/usenorn/norn/internal/repository/account"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	agentrepo "github.com/usenorn/norn/internal/repository/agent"
	agentproposalrepo "github.com/usenorn/norn/internal/repository/agentproposal"
	agentsettingrepo "github.com/usenorn/norn/internal/repository/agentsetting"
	apitokenrepo "github.com/usenorn/norn/internal/repository/apitoken"
	questionrepo "github.com/usenorn/norn/internal/repository/issuequestion"
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
	questions  *questionrepo.MockIssueQuestion
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
		questions:   questionrepo.NewMockIssueQuestion(ctrl),
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
		h.questions,
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

func expectActivePerson(h *harness, accountID uuid.UUID) {
	h.accounts.EXPECT().
		GetByID(gomock.Any(), accountID).
		Return(entity.Account{
			ID:     accountID,
			Kind:   entity.AccountKindPerson,
			Status: entity.AccountStatusActive,
		}, nil)
}

func expectActiveOwner(h *harness, accountID uuid.UUID) {
	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, accountID).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)
	expectActivePerson(h, accountID)
}

func TestRegisteringAnAgentCreatesANonHumanAccountThatCannotSignIn(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	var created entity.Account
	var createdAgent entity.Agent

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)
	expectActivePerson(h, h.adminID)

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
			createdAgent = agent

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

	if createdAgent.Icon != entity.AgentIconBot {
		t.Fatalf("default icon = %q, want bot", createdAgent.Icon)
	}

	if createdAgent.OwnerAccountID != h.adminID {
		t.Fatalf("owner account = %s, want current account %s", createdAgent.OwnerAccountID, h.adminID)
	}
}

func TestAnAgentIsRegisteredAsAViewerBecauseItsMembershipConfersNothing(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleAdmin}, nil)
	expectActivePerson(h, h.adminID)

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

func TestAnAgentCannotBeGivenMoreThanItsCurrentOwnerHas(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)
	expectActivePerson(h, h.adminID)

	_, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID: h.workspaceID,
		Name:        "triage-bot",
		Scopes:      adminOnlyScopes(),
		AllTeams:    true,
	})

	if !errors.Is(err, entity.ErrAPITokenScopeExceeds) {
		t.Fatalf(
			"Register error = %v, want ErrAPITokenScopeExceeds. An agent must not receive more "+
				"than the current person can do.",
			err,
		)
	}
}

func TestRegisteringAnAgentValidatesItsIconAndActionLimitBeforeWriting(t *testing.T) {
	limit := entity.AgentActionLimitMax + 1
	h := newHarness(t, entity.MembershipRoleAdmin)

	_, err := h.service.Register(context.Background(), service.RegisterAgentInput{
		WorkspaceID: h.workspaceID,
		Name:        "triage-bot",
		Icon:        "wand",
		Scopes:      readScopes(),
		AllTeams:    true,
		ActionLimit: &limit,
	})

	var validation entity.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Register error = %v, want ValidationError", err)
	}

	if len(validation.Fields) != 2 {
		t.Fatalf("validation fields = %+v, want icon and actionLimit", validation.Fields)
	}
}

func TestAnAgentOwnerMustBeAnActivePerson(t *testing.T) {
	now := time.Now().UTC()

	for _, test := range []struct {
		name       string
		membership entity.Membership
		account    *entity.Account
	}{
		{
			name:       "deactivated membership",
			membership: entity.Membership{Role: entity.MembershipRoleMember, DeactivatedAt: &now},
		},
		{
			name:       "agent account",
			membership: entity.Membership{Role: entity.MembershipRoleMember},
			account: &entity.Account{
				Kind:   entity.AccountKindAgent,
				Status: entity.AccountStatusActive,
			},
		},
		{
			name:       "integration account",
			membership: entity.Membership{Role: entity.MembershipRoleMember},
			account: &entity.Account{
				Kind:   entity.AccountKindIntegration,
				Status: entity.AccountStatusActive,
			},
		},
		{
			name:       "deactivated person",
			membership: entity.Membership{Role: entity.MembershipRoleMember},
			account: &entity.Account{
				Kind:   entity.AccountKindPerson,
				Status: entity.AccountStatusDeactivated,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t, entity.MembershipRoleAdmin)
			ownerID := h.adminID

			h.members.EXPECT().
				Get(gomock.Any(), h.workspaceID, ownerID).
				Return(test.membership, nil)

			if test.account != nil {
				test.account.ID = ownerID
				h.accounts.EXPECT().GetByID(gomock.Any(), ownerID).Return(*test.account, nil)
			}

			_, err := h.service.Register(context.Background(), service.RegisterAgentInput{
				WorkspaceID: h.workspaceID,
				Name:        "triage-bot",
				Scopes:      readScopes(),
				AllTeams:    true,
			})

			if !errors.Is(err, entity.ErrAgentOwnerInvalid) {
				t.Fatalf("Register error = %v, want ErrAgentOwnerInvalid", err)
			}
		})
	}
}

func TestDisablingAnAgentAlsoKillsItsCredentials(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID, agentAccount := uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, AccountID: agentAccount, Status: entity.AgentStatusActive}, nil)

	h.agents.EXPECT().Disable(gomock.Any(), h.workspaceID, agentID, gomock.Any()).Return(nil)
	h.members.EXPECT().
		SetDeactivated(gomock.Any(), h.workspaceID, agentAccount, gomock.Any()).
		Return(entity.Membership{}, nil)

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

func TestDisablingAnAgentTakesItsWorkspaceMembershipOutOfUse(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID, agentAccount := uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, AccountID: agentAccount, Status: entity.AgentStatusActive}, nil)

	h.agents.EXPECT().Disable(gomock.Any(), h.workspaceID, agentID, gomock.Any()).Return(nil)
	h.tokens.EXPECT().
		RevokeAllByAccount(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	var deactivated *time.Time

	h.members.EXPECT().
		SetDeactivated(gomock.Any(), h.workspaceID, agentAccount, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, at *time.Time) (entity.Membership, error) {
			deactivated = at

			return entity.Membership{DeactivatedAt: at}, nil
		})

	if err := h.service.Disable(context.Background(), h.workspaceID, agentID); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	if deactivated == nil {
		t.Fatal(
			"disabling an agent left its membership active. Every surface that offers agents " +
				"reads the membership, so a disabled one stays selectable.",
		)
	}
}

func TestEnablingAnAgentRestoresItsMembershipAndPreviousAuthority(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID, accountID, ownerID, teamID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	disabledAt := time.Now().UTC()
	agent := entity.Agent{
		ID:             agentID,
		WorkspaceID:    h.workspaceID,
		AccountID:      accountID,
		OwnerAccountID: ownerID,
		Name:           "opsy",
		Status:         entity.AgentStatusDisabled,
		DisabledAt:     &disabledAt,
	}
	scopes := entity.APIScopeSet{entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage)}
	grants := entity.APITokenGrants{{WorkspaceID: h.workspaceID, TeamIDs: []uuid.UUID{teamID}}}

	h.agents.EXPECT().GetByID(gomock.Any(), h.workspaceID, agentID).Return(agent, nil)
	expectActiveOwner(h, ownerID)
	h.tokens.EXPECT().
		GetLatestByOwner(gomock.Any(), accountID).
		Return(entity.APIToken{AccountID: accountID, Scopes: scopes, Grants: grants}, nil)
	h.agents.EXPECT().Enable(gomock.Any(), h.workspaceID, agentID).Return(nil)
	h.tokens.EXPECT().RevokeAllByAccount(gomock.Any(), accountID, gomock.Any()).Return(nil)

	membershipRestored := false
	h.members.EXPECT().
		SetDeactivated(gomock.Any(), h.workspaceID, accountID, nil).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, at *time.Time) (entity.Membership, error) {
			membershipRestored = at == nil

			return entity.Membership{}, nil
		})

	var minted entity.APIToken
	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			minted = token

			return token, nil
		})

	enabled, err := h.service.Enable(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	if enabled.Agent.Status != entity.AgentStatusActive || enabled.Agent.DisabledAt != nil {
		t.Fatalf("enabled agent = %+v, want active without disabledAt", enabled.Agent)
	}

	if !membershipRestored {
		t.Fatal("enabling an agent kept its membership deactivated")
	}

	if !strings.HasPrefix(enabled.Value, entity.APITokenPrefix) {
		t.Fatal("enabling an agent returned no fresh credential")
	}

	if len(minted.Scopes) != 1 || len(minted.Grants) != 1 ||
		len(minted.Grants[0].TeamIDs) != 1 || minted.Grants[0].TeamIDs[0] != teamID {
		t.Fatalf("minted authority = scopes %v grants %+v, want the previous authority", minted.Scopes, minted.Grants)
	}
}

func TestAnActiveAgentCannotBeEnabledAgain(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID := uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, Status: entity.AgentStatusActive}, nil)

	_, err := h.service.Enable(context.Background(), h.workspaceID, agentID)
	if !errors.Is(err, entity.ErrAgentActive) {
		t.Fatalf("Enable error = %v, want ErrAgentActive", err)
	}
}

func TestEnablingAnAgentWithoutRestorableAuthorityIsRefused(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID, accountID, ownerID := uuid.New(), uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:             agentID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Status:         entity.AgentStatusDisabled,
		}, nil)
	expectActiveOwner(h, ownerID)
	h.tokens.EXPECT().
		GetLatestByOwner(gomock.Any(), accountID).
		Return(entity.APIToken{}, entity.ErrAPITokenNotFound)

	_, err := h.service.Enable(context.Background(), h.workspaceID, agentID)
	if !errors.Is(err, entity.ErrAgentAuthorityMissing) {
		t.Fatalf("Enable error = %v, want ErrAgentAuthorityMissing", err)
	}
}

func TestConcurrentAgentEnableReturnsTheTypedActiveConflict(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID, accountID, ownerID := uuid.New(), uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:             agentID,
			WorkspaceID:    h.workspaceID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Status:         entity.AgentStatusDisabled,
		}, nil)
	expectActiveOwner(h, ownerID)
	h.tokens.EXPECT().
		GetLatestByOwner(gomock.Any(), accountID).
		Return(entity.APIToken{
			Scopes: readScopes(),
			Grants: entity.APITokenGrants{{WorkspaceID: h.workspaceID, AllTeams: true}},
		}, nil)
	h.agents.EXPECT().Enable(gomock.Any(), h.workspaceID, agentID).Return(entity.ErrAgentActive)

	_, err := h.service.Enable(context.Background(), h.workspaceID, agentID)
	if !errors.Is(err, entity.ErrAgentActive) {
		t.Fatalf("Enable error = %v, want ErrAgentActive", err)
	}
}

func TestAnAgentWithAnInvalidOwnerCannotBeReEnabled(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)
	agentID, accountID, ownerID := uuid.New(), uuid.New(), uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:             agentID,
			AccountID:      accountID,
			OwnerAccountID: ownerID,
			Status:         entity.AgentStatusDisabled,
		}, nil)
	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, ownerID).
		Return(entity.Membership{}, entity.ErrMembershipNotFound)

	_, err := h.service.Enable(context.Background(), h.workspaceID, agentID)
	if !errors.Is(err, entity.ErrAgentOwnerInvalid) {
		t.Fatalf("Enable error = %v, want ErrAgentOwnerInvalid", err)
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
		questionrepo.NewMockIssueQuestion(ctrl),
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

func TestRotatingACredentialRevokesTheOldOneAndKeepsWhatTheAgentMayDo(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID, accountID := uuid.New(), uuid.New()

	scopes := entity.APIScopeSet{
		entity.NewAPIScope(entity.ResourceIssue, entity.ActionManage),
	}
	grants := entity.APITokenGrants{{WorkspaceID: h.workspaceID, AllTeams: true}}

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:          agentID,
			WorkspaceID: h.workspaceID,
			AccountID:   accountID,
			Name:        "opsy",
			Status:      entity.AgentStatusActive,
		}, nil)

	h.tokens.EXPECT().
		ListByOwner(gomock.Any(), accountID).
		Return([]entity.APIToken{{AccountID: accountID, Scopes: scopes, Grants: grants}}, nil)

	revoked := false

	h.tokens.EXPECT().
		RevokeAllByAccount(gomock.Any(), accountID, gomock.Any()).
		DoAndReturn(func(context.Context, uuid.UUID, time.Time) error {
			revoked = true

			return nil
		})

	var minted entity.APIToken

	h.tokens.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token entity.APIToken) (entity.APIToken, error) {
			if !revoked {
				t.Error("a new credential was minted before the old one was revoked")
			}

			minted = token
			token.ID = uuid.New()

			return token, nil
		})

	rotated, err := h.service.Rotate(context.Background(), h.workspaceID, agentID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	if rotated.Value == "" {
		t.Fatal("rotation returned no credential, so there is nothing to give anybody")
	}

	if !revoked {
		t.Fatal(
			"the old credential was never revoked, so a leaked one keeps working after somebody " +
				"rotated precisely to stop it",
		)
	}

	if len(minted.Scopes) != len(scopes) {
		t.Fatalf(
			"the new credential carries %d scopes, want %d; rotating must not quietly change "+
				"what the agent may do",
			len(minted.Scopes), len(scopes),
		)
	}

	if len(minted.Grants) != 1 || !minted.Grants[0].AllTeams {
		t.Fatalf("the new credential lost the teams the old one reached: %+v", minted.Grants)
	}
}

func TestADisabledAgentCannotBeHandedAFreshCredential(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleAdmin)

	agentID := uuid.New()
	disabledAt := time.Now().UTC()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{
			ID:          agentID,
			WorkspaceID: h.workspaceID,
			AccountID:   uuid.New(),
			Status:      entity.AgentStatusDisabled,
			DisabledAt:  &disabledAt,
		}, nil)

	if _, err := h.service.Rotate(context.Background(), h.workspaceID, agentID); !errors.Is(
		err, entity.ErrAgentDisabled,
	) {
		t.Fatalf(
			"Rotate = %v, want the disabled agent refused; rotating one back to life would undo "+
				"a revocation somebody made deliberately",
			err,
		)
	}
}

func TestAMemberRegistersAnAgentThatActsForItself(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	var registered entity.Agent

	h.members.EXPECT().
		Get(gomock.Any(), h.workspaceID, h.adminID).
		Return(entity.Membership{Role: entity.MembershipRoleMember}, nil)
	expectActivePerson(h, h.adminID)

	h.accounts.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, account entity.Account) (entity.Account, error) {
			account.ID = uuid.New()

			return account, nil
		})

	h.members.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Membership{}, nil)
	h.agents.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, agent entity.Agent) (entity.Agent, error) {
			agent.ID = uuid.New()
			registered = agent

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

	if registered.OwnerAccountID != h.adminID {
		t.Fatal(
			"a member registered an agent and it was recorded against somebody else. An agent " +
				"carries its owner's authority, so the owner is the whole bound.",
		)
	}
}

func TestAMemberIsShownOnlyTheAgentsItActsFor(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	mine := entity.Agent{
		ID:             uuid.New(),
		WorkspaceID:    h.workspaceID,
		AccountID:      uuid.New(),
		OwnerAccountID: h.adminID,
		Name:           "mine",
	}
	theirs := entity.Agent{ID: uuid.New(), OwnerAccountID: uuid.New(), Name: "theirs"}

	h.agents.EXPECT().
		ListByWorkspaceID(gomock.Any(), h.workspaceID).
		Return([]entity.Agent{mine, theirs}, nil)

	h.accounts.EXPECT().
		GetByID(gomock.Any(), h.adminID).
		Return(entity.Account{ID: h.adminID, DisplayName: "Rae"}, nil)
	revokedAt := time.Now().UTC()
	h.tokens.EXPECT().
		GetLatestByOwner(gomock.Any(), mine.AccountID).
		Return(entity.APIToken{
			Scopes:    readScopes(),
			Grants:    entity.APITokenGrants{{WorkspaceID: h.workspaceID, AllTeams: true}},
			RevokedAt: &revokedAt,
		}, nil)

	listed, err := h.service.List(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(listed) != 1 || listed[0].Agent.ID != mine.ID {
		t.Fatalf(
			"a member was shown %d agents, want only the one it acts for. The list names the "+
				"owner of every agent, so somebody else's is somebody else's business.",
			len(listed),
		)
	}

	if !listed[0].Authority.AllTeams || len(listed[0].Authority.Scopes) != 1 {
		t.Fatalf("agent authority = %+v, want the latest credential's workspace authority", listed[0].Authority)
	}
}

func TestAnAgentSomebodyElseActsForIsNotThereToDisable(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	agentID := uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, OwnerAccountID: uuid.New()}, nil)

	if err := h.service.Disable(
		context.Background(), h.workspaceID, agentID,
	); !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf(
			"a member disabling an agent it does not act for: err = %v, want not found. "+
				"Refusing it any other way would confirm the agent exists.",
			err,
		)
	}
}

func TestAnAgentSomebodyElseActsForKeepsItsCredential(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	agentID := uuid.New()

	h.agents.EXPECT().
		GetByID(gomock.Any(), h.workspaceID, agentID).
		Return(entity.Agent{ID: agentID, OwnerAccountID: uuid.New()}, nil)

	if _, err := h.service.Rotate(
		context.Background(), h.workspaceID, agentID,
	); !errors.Is(err, entity.ErrAgentNotFound) {
		t.Fatalf(
			"a member rotated the credential of an agent it does not act for: err = %v, want "+
				"not found",
			err,
		)
	}
}

func TestAMemberDoesNotRatifyWhatATeamHeld(t *testing.T) {
	h := newHarness(t, entity.MembershipRoleMember)

	if _, err := h.service.Waiting(
		context.Background(), h.workspaceID,
	); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf("a member reading the approval queue: err = %v, want forbidden", err)
	}

	if _, err := h.service.Approve(
		context.Background(), h.workspaceID, uuid.New(),
	); !errors.Is(err, entity.ErrAccountForbidden) {
		t.Fatalf(
			"a member approved a held write: err = %v, want forbidden. A team holds an agent's "+
				"writes so that somebody who runs the workspace agrees to them, and letting a "+
				"member registering agents also ratify them would answer the hold with itself.",
			err,
		)
	}
}
