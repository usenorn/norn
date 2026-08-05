package entity

import (
	"slices"
	"strings"
)

const apiScopeSeparator = ":"

type APIScope string

func NewAPIScope(resource Resource, action Action) APIScope {
	return APIScope(string(resource) + apiScopeSeparator + string(action))
}

func (s APIScope) Resource() Resource {
	resource, _, _ := strings.Cut(string(s), apiScopeSeparator)

	return Resource(resource)
}

func (s APIScope) Action() Action {
	_, action, _ := strings.Cut(string(s), apiScopeSeparator)

	return Action(action)
}

func (s APIScope) Valid() bool {
	return slices.Contains(apiScopeCatalog, s)
}

var apiScopeCatalog = []APIScope{
	NewAPIScope(ResourceWorkspace, ActionRead),
	NewAPIScope(ResourceWorkspace, ActionUpdate),
	NewAPIScope(ResourceMembership, ActionRead),
	NewAPIScope(ResourceMembership, ActionManage),
	NewAPIScope(ResourceInvitation, ActionRead),
	NewAPIScope(ResourceInvitation, ActionManage),
	NewAPIScope(ResourceTeam, ActionRead),
	NewAPIScope(ResourceTeam, ActionManage),
	NewAPIScope(ResourceTeamMembership, ActionRead),
	NewAPIScope(ResourceTeamMembership, ActionManage),
	NewAPIScope(ResourceIssue, ActionRead),
	NewAPIScope(ResourceIssue, ActionManage),
	NewAPIScope(ResourceLabel, ActionRead),
	NewAPIScope(ResourceLabel, ActionManage),
	NewAPIScope(ResourceProject, ActionRead),
	NewAPIScope(ResourceProject, ActionManage),
}

func APIScopes() []APIScope {
	return slices.Clone(apiScopeCatalog)
}

type APIScopeSet []APIScope

func (s APIScopeSet) Permits(resource Resource, action Action) bool {
	return slices.Contains(s, NewAPIScope(resource, action))
}

func (s APIScopeSet) Normalized() APIScopeSet {
	normalized := slices.Clone(s)
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)

	return slices.DeleteFunc(normalized, func(scope APIScope) bool { return !scope.Valid() })
}

func (s APIScopeSet) SubsetOf(other APIScopeSet) bool {
	for _, scope := range s {
		if !slices.Contains(other, scope) {
			return false
		}
	}

	return true
}

func (s APIScopeSet) Strings() []string {
	values := make([]string, 0, len(s))

	for _, scope := range s {
		values = append(values, string(scope))
	}

	return values
}

func NewAPIScopeSet(values []string) APIScopeSet {
	scopes := make(APIScopeSet, 0, len(values))

	for _, value := range values {
		scopes = append(scopes, APIScope(value))
	}

	return scopes
}

// AllowedAPIScopesFor is the ceiling a token may be minted with. It reads the same rule the runtime
// policy is built from, so a token can be granted exactly what its creator may do — no more, and
// just as importantly no less, which a second hand-maintained rule set would drift into.
func AllowedAPIScopesFor(role MembershipRole) APIScopeSet {
	allowed := make(APIScopeSet, 0, len(apiScopeCatalog))

	for _, scope := range apiScopeCatalog {
		if RoleGrants(role, scope.Resource(), scope.Action()) {
			allowed = append(allowed, scope)
		}
	}

	return allowed
}

// RoleGrants is the authorization policy: what a role may do to a resource. The casbin enforcer is
// built from it and the token ceiling is derived from it, so the two cannot disagree.
func RoleGrants(role MembershipRole, resource Resource, action Action) bool {
	if resource == ResourceWorkspace && action == ActionDelete {
		return role == MembershipRoleAdmin
	}

	// Managing tokens in a workspace context is administrative oversight of other people's
	// credentials. Acting on your own tokens goes through the self branch and never lands here.
	if resource == ResourceAPIToken {
		return role == MembershipRoleAdmin
	}

	if action == ActionUpdate && resource != ResourceWorkspace {
		return false
	}

	if action == ActionDelete {
		return false
	}

	if resource == ResourceProject && action == ActionManage {
		return role != MembershipRoleViewer
	}

	if resource == ResourceSavedView && action == ActionManage {
		return true
	}

	if resource == ResourceComment && action == ActionManage {
		return true
	}

	if resource == ResourceNotification {
		return action == ActionRead || action == ActionManage
	}

	return RolePermits(role, resource, action)
}

func RolePermits(role MembershipRole, resource Resource, action Action) bool {
	if !role.Valid() {
		return false
	}

	if action == ActionRead {
		return true
	}

	return role == MembershipRoleAdmin
}
