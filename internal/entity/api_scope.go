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

func AllowedAPIScopesFor(role MembershipRole) APIScopeSet {
	allowed := make(APIScopeSet, 0, len(apiScopeCatalog))

	for _, scope := range apiScopeCatalog {
		if RolePermits(role, scope.Resource(), scope.Action()) {
			allowed = append(allowed, scope)
		}
	}

	return allowed
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
