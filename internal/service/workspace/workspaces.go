package workspace

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type workspacesService struct {
	workspaces   repository.Workspace
	memberships  repository.Membership
	accounts     repository.Account
	authPolicies repository.WorkspaceAuthPolicy
	authorizer   service.Authorizer
	transactor   repository.Transactor
}

func New(
	workspaces repository.Workspace,
	memberships repository.Membership,
	accounts repository.Account,
	authPolicies repository.WorkspaceAuthPolicy,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Workspaces {
	return &workspacesService{
		workspaces:   workspaces,
		memberships:  memberships,
		accounts:     accounts,
		authPolicies: authPolicies,
		authorizer:   authorizer,
		transactor:   transactor,
	}
}

func (s *workspacesService) Create(ctx context.Context, input service.CreateWorkspaceInput) (entity.Workspace, error) {
	actor, ok := identity.From(ctx)
	if !ok {
		return entity.Workspace{}, entity.ErrAccountForbidden
	}

	if err := entity.NewValidationError(
		entity.ValidateWorkspaceSlug("slug", input.Slug),
		entity.ValidateWorkspaceName("name", input.Name),
	); err != nil {
		return entity.Workspace{}, err
	}

	var workspace entity.Workspace

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err := s.workspaces.Create(ctx, entity.Workspace{Slug: input.Slug, Name: input.Name})
		if err != nil {
			return err
		}

		if _, err := s.memberships.Create(ctx, entity.Membership{
			WorkspaceID: created.ID,
			AccountID:   actor,
			Role:        entity.MembershipRoleAdmin,
		}); err != nil {
			return err
		}

		workspace = created

		return nil
	})
	if err != nil {
		return entity.Workspace{}, err
	}

	return workspace, nil
}

func (s *workspacesService) ListForAccount(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error) {
	actor, ok := identity.From(ctx)
	if !ok || actor != accountID {
		return nil, entity.ErrAccountForbidden
	}

	return s.workspaces.ListByAccountID(ctx, accountID)
}

func (s *workspacesService) ListMembers(ctx context.Context, workspaceID uuid.UUID) ([]entity.Membership, error) {
	if err := s.authorizeActor(ctx, workspaceID, entity.ResourceMembership, entity.ActionRead); err != nil {
		return nil, err
	}

	return s.memberships.ListByWorkspaceID(ctx, workspaceID)
}

func (s *workspacesService) AddMember(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error) {
	if err := s.authorizeActor(ctx, workspaceID, entity.ResourceMembership, entity.ActionManage); err != nil {
		return entity.Membership{}, err
	}

	if !role.Valid() {
		return entity.Membership{}, entity.NewValidationError(entity.FieldError{
			Field: "role",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return entity.Membership{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.Membership{}, entity.ErrAccountDeactivated
	}

	return s.memberships.Create(ctx, entity.Membership{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Role:        role,
	})
}

func (s *workspacesService) ChangeMemberRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error) {
	if err := s.authorizeActor(ctx, workspaceID, entity.ResourceMembership, entity.ActionManage); err != nil {
		return entity.Membership{}, err
	}

	if !role.Valid() {
		return entity.Membership{}, entity.NewValidationError(entity.FieldError{
			Field: "role",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	var membership entity.Membership

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if role != entity.MembershipRoleAdmin {
			if err := s.guardLastAdmin(ctx, workspaceID, accountID); err != nil {
				return err
			}
		}

		updated, err := s.memberships.UpdateRole(ctx, workspaceID, accountID, role)
		if err != nil {
			return err
		}

		membership = updated

		return nil
	})
	if err != nil {
		return entity.Membership{}, err
	}

	return membership, nil
}

func (s *workspacesService) RemoveMember(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	if err := s.authorizeActor(ctx, workspaceID, entity.ResourceMembership, entity.ActionManage); err != nil {
		return err
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.guardLastAdmin(ctx, workspaceID, accountID); err != nil {
			return err
		}

		return s.memberships.Delete(ctx, workspaceID, accountID)
	})
}

func (s *workspacesService) guardLastAdmin(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
		return err
	}

	soleAdminWorkspaceIDs, err := s.memberships.ListWorkspaceIDsWithoutOtherActiveAdmin(ctx, accountID)
	if err != nil {
		return err
	}

	if slices.Contains(soleAdminWorkspaceIDs, workspaceID) {
		return entity.LastWorkspaceAdminError{WorkspaceIDs: []uuid.UUID{workspaceID}}
	}

	return nil
}

func (s *workspacesService) authorizeActor(ctx context.Context, workspaceID uuid.UUID, resource, action string) error {
	membership, err := s.actorMembership(ctx, workspaceID)
	if err != nil {
		return err
	}

	if err := s.narrowByAuthMethod(ctx, workspaceID); err != nil {
		return err
	}

	return s.authorizer.Authorize(ctx, membership.Role, resource, action)
}

func (s *workspacesService) actorMembership(ctx context.Context, workspaceID uuid.UUID) (entity.Membership, error) {
	actor, ok := identity.From(ctx)
	if !ok {
		return entity.Membership{}, entity.ErrAccountForbidden
	}

	membership, err := s.memberships.Get(ctx, workspaceID, actor)
	if err != nil {
		return entity.Membership{}, entity.ErrAccountForbidden
	}

	return membership, nil
}

func (s *workspacesService) narrowByAuthMethod(ctx context.Context, workspaceID uuid.UUID) error {
	session, ok := identity.CurrentSession(ctx)
	if !ok {
		return entity.ErrAccountForbidden
	}

	policy, err := s.authPolicies.Get(ctx, workspaceID)
	if err != nil {
		return err
	}

	if !policy.Enforcement.Permits(session.AuthMethod) {
		return entity.ErrWorkspaceAuthMethodNotPermitted
	}

	return nil
}

func (s *workspacesService) AuthPolicy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
	membership, err := s.actorMembership(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	if err := s.authorizer.Authorize(ctx, membership.Role, entity.ResourceWorkspace, entity.ActionRead); err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	return s.authPolicies.Get(ctx, workspaceID)
}

func (s *workspacesService) SetAuthPolicy(ctx context.Context, workspaceID uuid.UUID, enforcement entity.AuthEnforcement) (entity.WorkspaceAuthPolicy, error) {
	membership, err := s.actorMembership(ctx, workspaceID)
	if err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	if err := s.authorizer.Authorize(ctx, membership.Role, entity.ResourceWorkspace, entity.ActionUpdate); err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	if !enforcement.Valid() {
		return entity.WorkspaceAuthPolicy{}, entity.NewValidationError(entity.FieldError{
			Field: "enforcement",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return s.authPolicies.Upsert(ctx, entity.WorkspaceAuthPolicy{
		WorkspaceID: workspaceID,
		Enforcement: enforcement,
	})
}
