package authorizer

import (
	"context"
	"fmt"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/authz"
	"github.com/usenorn/norn/internal/service"
)

var policy = [][]string{
	{string(entity.MembershipRoleAdmin), entity.ResourceWorkspace, entity.ActionRead},
	{string(entity.MembershipRoleAdmin), entity.ResourceWorkspace, entity.ActionUpdate},
	{string(entity.MembershipRoleAdmin), entity.ResourceWorkspace, entity.ActionDelete},
	{string(entity.MembershipRoleAdmin), entity.ResourceMembership, entity.ActionRead},
	{string(entity.MembershipRoleAdmin), entity.ResourceMembership, entity.ActionManage},
	{string(entity.MembershipRoleMember), entity.ResourceWorkspace, entity.ActionRead},
	{string(entity.MembershipRoleMember), entity.ResourceMembership, entity.ActionRead},
}

type casbinAuthorizer struct {
	enforcer *authz.Enforcer
}

func New(enforcer *authz.Enforcer) service.Authorizer {
	return &casbinAuthorizer{enforcer: enforcer}
}

func (a *casbinAuthorizer) Authorize(_ context.Context, role entity.MembershipRole, resource, action string) error {
	if !role.Valid() {
		return entity.ErrAccountForbidden
	}

	allowed, err := a.enforcer.Enforce(string(role), resource, action)
	if err != nil {
		return fmt.Errorf("evaluate authorization policy: %w", err)
	}

	if !allowed {
		return entity.ErrAccountForbidden
	}

	return nil
}

func (a *casbinAuthorizer) SeedPolicy(_ context.Context) error {
	rules := make([][]string, 0, len(policy))

	for _, rule := range policy {
		exists, err := a.enforcer.HasPolicy(rule[0], rule[1], rule[2])
		if err != nil {
			return fmt.Errorf("check authorization policy: %w", err)
		}

		if !exists {
			rules = append(rules, rule)
		}
	}

	if len(rules) == 0 {
		return nil
	}

	if _, err := a.enforcer.AddPolicies(rules); err != nil {
		return fmt.Errorf("seed authorization policy: %w", err)
	}

	return nil
}
