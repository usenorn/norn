package agent

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type agentsService struct {
	agents      repository.Agent
	settings    repository.AgentSetting
	proposals   repository.AgentProposal
	accounts    repository.Account
	memberships repository.Membership
	tokens      repository.APIToken
	teams       repository.Team
	activity    repository.Activity
	states      repository.WorkflowState
	issues      service.Issues
	comments    service.IssueComments
	checks      service.Checks
	authorizer  service.Authorizer
	transactor  repository.Transactor
	audit       service.Audit
}

func New(
	agents repository.Agent,
	settings repository.AgentSetting,
	proposals repository.AgentProposal,
	accounts repository.Account,
	memberships repository.Membership,
	tokens repository.APIToken,
	teams repository.Team,
	activity repository.Activity,
	states repository.WorkflowState,
	issues service.Issues,
	comments service.IssueComments,
	checks service.Checks,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	audit service.Audit,
) service.Agents {
	return &agentsService{
		agents:      agents,
		settings:    settings,
		proposals:   proposals,
		accounts:    accounts,
		memberships: memberships,
		tokens:      tokens,
		teams:       teams,
		activity:    activity,
		states:      states,
		issues:      issues,
		comments:    comments,
		checks:      checks,
		authorizer:  authorizer,
		transactor:  transactor,
		audit:       audit,
	}
}

func (s *agentsService) Register(
	ctx context.Context,
	input service.RegisterAgentInput,
) (service.RegisteredAgent, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.RegisteredAgent{}, err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return service.RegisteredAgent{}, entity.ErrAPITokenMintForbidden
	}

	if err := entity.NewValidationError(entity.ValidateAgentName("name", input.Name)); err != nil {
		return service.RegisteredAgent{}, err
	}

	owner := input.OwnerAccountID
	if owner == uuid.Nil {
		owner = decision.Actor.AccountID
	}

	ownership, err := s.memberships.Get(ctx, input.WorkspaceID, owner)
	if err != nil {
		return service.RegisteredAgent{}, entity.ErrAgentOwnerInvalid
	}

	scopes := input.Scopes.Normalized()

	if len(scopes) == 0 || len(scopes) != len(input.Scopes) {
		return service.RegisteredAgent{}, entity.ErrAPITokenScopeInvalid
	}

	if !scopes.SubsetOf(entity.AllowedAPIScopesFor(ownership.Role)) {
		return service.RegisteredAgent{}, entity.ErrAPITokenScopeExceeds
	}

	grant, err := s.grant(ctx, input, decision)
	if err != nil {
		return service.RegisteredAgent{}, err
	}

	value, tokenHash, err := entity.NewAPIToken()
	if err != nil {
		return service.RegisteredAgent{}, err
	}

	var registered service.RegisteredAgent

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		account, err := s.accounts.Create(ctx, entity.Account{
			Status:      entity.AccountStatusActive,
			Kind:        entity.AccountKindAgent,
			DisplayName: input.Name,
			Timezone:    decision.Workspace.Timezone,
		})
		if err != nil {
			return err
		}

		if _, err := s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: input.WorkspaceID,
			AccountID:   account.ID,
			Role:        entity.MembershipRoleViewer,
			Source:      entity.MembershipSourceManual,
		}); err != nil {
			return err
		}

		agent, err := s.agents.Create(ctx, entity.Agent{
			WorkspaceID:    input.WorkspaceID,
			AccountID:      account.ID,
			OwnerAccountID: owner,
			Name:           input.Name,
			ActionLimit:    input.ActionLimit,
		})
		if err != nil {
			return err
		}

		expiresAt := time.Now().UTC().Add(entity.APITokenMaxTTL)

		token, err := s.tokens.Create(ctx, entity.APIToken{
			AccountID: account.ID,
			Name:      input.Name,
			TokenHash: tokenHash,
			Scopes:    scopes,
			Grants:    entity.APITokenGrants{grant},
			ExpiresAt: &expiresAt,
		})
		if err != nil {
			return err
		}

		registered = service.RegisteredAgent{Agent: agent, Token: token, Value: value}

		return nil
	}); err != nil {
		return service.RegisteredAgent{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  input.WorkspaceID,
		Action:       entity.AuditAgentRegistered,
		ResourceKind: string(entity.ResourceAgent),
		ResourceID:   registered.Agent.ID,
		ResourceName: input.Name,
	})

	return registered, nil
}

func (s *agentsService) grant(
	ctx context.Context,
	input service.RegisterAgentInput,
	decision entity.Decision,
) (entity.APITokenGrant, error) {
	if input.AllTeams {
		return entity.APITokenGrant{WorkspaceID: input.WorkspaceID, AllTeams: true}, nil
	}

	if len(input.TeamIDs) == 0 {
		return entity.APITokenGrant{}, entity.ErrAPITokenGrantInvalid
	}

	for _, teamID := range input.TeamIDs {
		if !decision.Scope.Covers(teamID) {
			return entity.APITokenGrant{}, entity.ErrAPITokenGrantInvalid
		}
	}

	return entity.APITokenGrant{WorkspaceID: input.WorkspaceID, TeamIDs: input.TeamIDs}, nil
}

func (s *agentsService) List(ctx context.Context, workspaceID uuid.UUID) ([]service.OwnedAgent, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return nil, err
	}

	agents, err := s.agents.ListByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	owned := make([]service.OwnedAgent, 0, len(agents))

	for _, agent := range agents {
		described, err := s.describe(ctx, agent)
		if err != nil {
			return nil, err
		}

		owned = append(owned, described)
	}

	return owned, nil
}

func (s *agentsService) Get(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
) (service.OwnedAgent, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return service.OwnedAgent{}, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return service.OwnedAgent{}, err
	}

	return s.describe(ctx, agent)
}

func (s *agentsService) describe(ctx context.Context, agent entity.Agent) (service.OwnedAgent, error) {
	owner, err := s.accounts.GetByID(ctx, agent.OwnerAccountID)
	if err != nil {
		return service.OwnedAgent{}, err
	}

	return service.OwnedAgent{
		Agent:      agent,
		OwnerName:  owner.DisplayName,
		OwnerEmail: owner.Email,
	}, nil
}

func (s *agentsService) Disable(ctx context.Context, workspaceID, agentID uuid.UUID) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return err
	}

	if decision.Actor.Kind != entity.ActorKindUser {
		return entity.ErrAPITokenMintForbidden
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.agents.Disable(ctx, workspaceID, agentID, now); err != nil {
			return err
		}

		return s.tokens.RevokeAllByAccount(ctx, agent.AccountID, now)
	}); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditAgentDisabled,
		ResourceKind: string(entity.ResourceAgent),
		ResourceID:   agentID,
		ResourceName: agent.Name,
	})

	return nil
}

func (s *agentsService) Activity(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
	page entity.ActivityPage,
) (service.ActivityPage, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAgent,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return service.ActivityPage{}, err
	}

	agent, err := s.agents.GetByID(ctx, workspaceID, agentID)
	if err != nil {
		return service.ActivityPage{}, err
	}

	page = page.Normalized()

	events, err := s.activity.ListByActor(ctx, workspaceID, agent.AccountID, page.Lookahead())
	if err != nil {
		return service.ActivityPage{}, err
	}

	result := service.ActivityPage{Events: events}

	if len(events) > page.Limit {
		result.Events = events[:page.Limit]
		result.NextCursor = result.Events[len(result.Events)-1].Cursor().Encode()
	}

	return result, nil
}

func (s *agentsService) Authenticate(
	ctx context.Context,
	accountID uuid.UUID,
	actor entity.Actor,
) (entity.Actor, error) {
	agent, err := s.agents.GetByAccountID(ctx, accountID)
	if err != nil {
		return entity.Actor{}, err
	}

	if agent.Disabled() {
		return entity.Actor{}, entity.ErrAgentDisabled
	}

	agentID := agent.ID

	actor.AgentID = &agentID
	actor.AgentAllowance = agent.Allowance()
	actor.OwnerAccountID = agent.OwnerAccountID

	return actor, nil
}
