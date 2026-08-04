package authorizer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/observability/logging"
	"github.com/usenorn/norn/internal/pkg/authz"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

var policyResources = []entity.Resource{
	entity.ResourceWorkspace,
	entity.ResourceMembership,
	entity.ResourceInvitation,
	entity.ResourceTeam,
	entity.ResourceTeamMembership,
	entity.ResourceIssue,
	entity.ResourceCycle,
	entity.ResourceLabel,
	entity.ResourceAPIToken,
}

var policyActions = []entity.Action{
	entity.ActionRead,
	entity.ActionUpdate,
	entity.ActionDelete,
	entity.ActionManage,
}

var policyRoles = []entity.MembershipRole{
	entity.MembershipRoleAdmin,
	entity.MembershipRoleMember,
	entity.MembershipRoleViewer,
}

func policyRules() [][]string {
	rules := make([][]string, 0, len(policyRoles)*len(policyResources))

	for _, role := range policyRoles {
		for _, resource := range policyResources {
			for _, action := range policyActions {
				if !grants(role, resource, action) {
					continue
				}

				rules = append(rules, []string{string(role), string(resource), string(action)})
			}
		}
	}

	return rules
}

func grants(role entity.MembershipRole, resource entity.Resource, action entity.Action) bool {
	if resource == entity.ResourceWorkspace && action == entity.ActionDelete {
		return role == entity.MembershipRoleAdmin
	}

	if resource == entity.ResourceAPIToken {
		return role != entity.MembershipRoleViewer
	}

	if action == entity.ActionUpdate && resource != entity.ResourceWorkspace {
		return false
	}

	if action == entity.ActionDelete {
		return false
	}

	return entity.RolePermits(role, resource, action)
}

type policyEnforcer interface {
	Enforce(rvals ...any) (bool, error)
	GetPolicy() ([][]string, error)
	AddPolicies(rules [][]string) (bool, error)
	RemovePolicies(rules [][]string) (bool, error)
}

type casbinAuthorizer struct {
	enforcer     policyEnforcer
	memberships  repository.Membership
	authPolicies repository.WorkspaceAuthPolicy
	workspaces   repository.Workspace
	teams        repository.Team
	accounts     repository.Account
}

func New(
	enforcer *authz.Enforcer,
	memberships repository.Membership,
	authPolicies repository.WorkspaceAuthPolicy,
	workspaces repository.Workspace,
	teams repository.Team,
	accounts repository.Account,
) service.Authorizer {
	return &casbinAuthorizer{
		enforcer:     enforcer,
		memberships:  memberships,
		authPolicies: authPolicies,
		workspaces:   workspaces,
		teams:        teams,
		accounts:     accounts,
	}
}

func (a *casbinAuthorizer) deny(
	ctx context.Context,
	actor entity.Actor,
	request entity.AccessRequest,
	reason entity.DenyReason,
) entity.AccessDeniedError {
	denied := entity.AccessDeniedError{
		Reason:      reason,
		Resource:    request.Resource,
		Action:      request.Action,
		WorkspaceID: request.WorkspaceID,
		ActorKind:   actor.Kind,
		AccountID:   actor.AccountID,
	}

	if actor.TokenID != nil {
		denied.TokenID = *actor.TokenID
	}

	attrs := []any{
		"reason", string(reason),
		"resource", string(request.Resource),
		"action", string(request.Action),
		"actor_kind", string(actor.Kind),
	}

	if request.WorkspaceID != uuid.Nil {
		attrs = append(attrs, "workspace_id", request.WorkspaceID.String())
	}

	logging.From(ctx).InfoContext(ctx, "access denied", attrs...)

	return denied
}

func (a *casbinAuthorizer) Decide(ctx context.Context, request entity.AccessRequest) (entity.Decision, error) {
	actor, ok := identity.Actor(ctx)

	if !ok && !request.Joining {
		return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonNoActor)
	}

	if !actor.Holds(entity.NewPermission(request.Resource, request.Action)) {
		return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonTokenPermissionMissing)
	}

	switch {
	case request.WorkspaceID != uuid.Nil:
		return a.decideInWorkspace(ctx, actor, request)

	case request.Subject != uuid.Nil:
		if actor.AccountID != request.Subject {
			return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonNotSelf)
		}

		return entity.Decision{Actor: actor}, nil

	case request.Resource == entity.ResourceInstance:
		admin, err := a.instanceAdmin(ctx, actor)
		if err != nil {
			return entity.Decision{}, err
		}

		if !admin {
			return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonInstanceAdminRequired)
		}

		actor.InstanceAdmin = true

		return entity.Decision{Actor: actor}, nil

	default:
		return entity.Decision{Actor: actor}, nil
	}
}

func (a *casbinAuthorizer) instanceAdmin(ctx context.Context, actor entity.Actor) (bool, error) {
	if actor.Kind != entity.ActorKindUser {
		return false, nil
	}

	account, err := a.accounts.GetByID(ctx, actor.AccountID)
	if err != nil {
		if errors.Is(err, entity.ErrAccountNotFound) {
			return false, nil
		}

		return false, err
	}

	return account.InstanceAdmin, nil
}

func (a *casbinAuthorizer) decideInWorkspace(
	ctx context.Context,
	actor entity.Actor,
	request entity.AccessRequest,
) (entity.Decision, error) {
	if !actor.ConfinedTo(request.WorkspaceID) {
		return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonTokenWorkspaceMismatch)
	}

	decision := entity.Decision{Actor: actor}

	if !request.Joining {
		membership, err := a.memberships.Get(ctx, request.WorkspaceID, actor.AccountID)
		if err != nil {
			if errors.Is(err, entity.ErrMembershipNotFound) {
				return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonNotAMember)
			}

			return entity.Decision{}, err
		}

		decision.Role = membership.Role
	}

	if request.Resource.GatedByAuthMethod() {
		policy, err := a.authPolicies.Get(ctx, request.WorkspaceID)
		if err != nil {
			return entity.Decision{}, err
		}

		if !policy.Enforcement.PermitsActor(actor) {
			return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonAuthMethodNotPermitted)
		}
	}

	if !request.Joining {
		if !decision.Role.Valid() {
			return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonUnknownRole)
		}

		allowed, err := a.enforcer.Enforce(
			string(decision.Role),
			request.Resource.PolicyName(),
			string(request.Action),
		)
		if err != nil {
			return entity.Decision{}, fmt.Errorf("evaluate authorization policy: %w", err)
		}

		if !allowed {
			return entity.Decision{}, a.deny(ctx, actor, request, entity.DenyReasonRoleLacksAction)
		}
	}

	workspace, err := a.workspaces.GetByID(ctx, request.WorkspaceID)
	if err != nil {
		return entity.Decision{}, err
	}

	if workspace.Deleted() && request.Action.RequiresLiveWorkspace() {
		denied := a.deny(ctx, actor, request, entity.DenyReasonWorkspaceDeleted)
		denied.PurgeAfter = workspace.PurgeAfter

		return entity.Decision{}, denied
	}

	decision.Workspace = workspace

	if request.Scoped {
		scope, err := a.teamScope(ctx, actor, request.WorkspaceID, decision.Role)
		if err != nil {
			return entity.Decision{}, err
		}

		decision.Scope = scope
	}

	return decision, nil
}

func (a *casbinAuthorizer) teamScope(
	ctx context.Context,
	actor entity.Actor,
	workspaceID uuid.UUID,
	role entity.MembershipRole,
) (entity.TeamScope, error) {
	scope := entity.TeamScope{WorkspaceID: workspaceID, TeamIDs: []uuid.UUID{}}

	if role == entity.MembershipRoleAdmin {
		scope.AllTeams = true

		return scope, nil
	}

	teams, err := a.teams.ListVisibleTo(ctx, workspaceID, actor.AccountID, "", false)
	if err != nil {
		return entity.TeamScope{}, err
	}

	for _, team := range teams {
		scope.TeamIDs = append(scope.TeamIDs, team.ID)
	}

	return scope, nil
}

func (a *casbinAuthorizer) SeedPolicy(_ context.Context) error {
	desired := policyRules()

	existing, err := a.enforcer.GetPolicy()
	if err != nil {
		return fmt.Errorf("read authorization policy: %w", err)
	}

	stale := make([][]string, 0, len(existing))

	for _, rule := range existing {
		if !containsRule(desired, rule) {
			stale = append(stale, rule)
		}
	}

	if len(stale) > 0 {
		if _, err := a.enforcer.RemovePolicies(stale); err != nil {
			return fmt.Errorf("prune authorization policy: %w", err)
		}
	}

	missing := make([][]string, 0, len(desired))

	for _, rule := range desired {
		if !containsRule(existing, rule) {
			missing = append(missing, rule)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if _, err := a.enforcer.AddPolicies(missing); err != nil {
		return fmt.Errorf("seed authorization policy: %w", err)
	}

	return nil
}

func containsRule(rules [][]string, rule []string) bool {
	for _, candidate := range rules {
		if len(candidate) != len(rule) {
			continue
		}

		match := true

		for i := range rule {
			if candidate[i] != rule[i] {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
