package workspace

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/config"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type workspacesService struct {
	workspaces   repository.Workspace
	memberships  repository.Membership
	accounts     repository.Account
	teams        repository.Team
	teamMembers  repository.TeamMember
	states       repository.WorkflowState
	authPolicies repository.WorkspaceAuthPolicy
	connections  repository.SSOConnection
	identities   repository.SSOIdentity
	breakGlass   repository.BreakGlass
	producer     repository.JobProducer
	blobs        repository.Blob
	authorizer   service.Authorizer
	transactor   repository.Transactor
	workspaceCfg config.Workspace
	audit        service.Audit
}

func New(
	workspaces repository.Workspace,
	memberships repository.Membership,
	accounts repository.Account,
	teams repository.Team,
	teamMembers repository.TeamMember,
	states repository.WorkflowState,
	authPolicies repository.WorkspaceAuthPolicy,
	connections repository.SSOConnection,
	identities repository.SSOIdentity,
	breakGlass repository.BreakGlass,
	producer repository.JobProducer,
	blobs repository.Blob,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	workspaceCfg config.Workspace,
	audit service.Audit,
) service.Workspaces {
	return &workspacesService{
		workspaces:   workspaces,
		memberships:  memberships,
		accounts:     accounts,
		teams:        teams,
		teamMembers:  teamMembers,
		states:       states,
		authPolicies: authPolicies,
		connections:  connections,
		identities:   identities,
		breakGlass:   breakGlass,
		producer:     producer,
		blobs:        blobs,
		authorizer:   authorizer,
		transactor:   transactor,
		workspaceCfg: workspaceCfg,
		audit:        audit,
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
			Source:      entity.MembershipSourceManual,
			ReadsAudit:  true,
		}); err != nil {
			return err
		}

		if input.Team != nil {
			created, err = s.seedFirstTeam(ctx, created, actor, *input.Team)
			if err != nil {
				return err
			}
		}

		workspace = created

		return nil
	})
	if err != nil {
		return entity.Workspace{}, err
	}

	return workspace, nil
}

func (s *workspacesService) seedFirstTeam(
	ctx context.Context,
	workspace entity.Workspace,
	actor uuid.UUID,
	input service.CreateWorkspaceTeamInput,
) (entity.Workspace, error) {
	team, err := s.teams.Create(ctx, entity.Team{
		WorkspaceID: workspace.ID,
		Key:         input.Key,
		Name:        input.Name,
		Status:      entity.TeamStatusActive,
		Visibility:  entity.DefaultTeamVisibility,
	})
	if err != nil {
		return entity.Workspace{}, err
	}

	if _, err := s.states.CreateMany(ctx, entity.DefaultWorkflowStates(workspace.ID, team.ID)); err != nil {
		return entity.Workspace{}, err
	}

	if _, err := s.teamMembers.Create(ctx, entity.TeamMembership{
		WorkspaceID: workspace.ID,
		TeamID:      team.ID,
		AccountID:   actor,
	}); err != nil {
		return entity.Workspace{}, err
	}

	return s.workspaces.UpdateSettings(ctx, workspace.ID, workspace.Name, workspace.Timezone, &team.ID)
}

func (s *workspacesService) Get(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionRead, WorkspaceID: workspaceID})
	if err != nil {
		return entity.Workspace{}, err
	}

	return decision.Workspace, nil
}

func (s *workspacesService) Update(ctx context.Context, workspaceID uuid.UUID, input service.UpdateWorkspaceInput) (entity.Workspace, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionUpdate, WorkspaceID: workspaceID})
	if err != nil {
		return entity.Workspace{}, err
	}

	current := decision.Workspace

	name := current.Name
	if input.Name != nil {
		name = *input.Name
	}

	timezone := current.Timezone
	if input.Timezone != nil {
		timezone = *input.Timezone
	}

	if err := entity.NewValidationError(
		entity.ValidateWorkspaceName("name", name),
		entity.ValidateTimezone("timezone", timezone),
	); err != nil {
		return entity.Workspace{}, err
	}

	defaultTeamID := current.DefaultTeamID

	if input.DefaultTeamID != nil {
		assignable, err := s.assignableTeam(ctx, workspaceID, *input.DefaultTeamID)
		if err != nil {
			return entity.Workspace{}, err
		}

		defaultTeamID = &assignable
	}

	updated, err := s.workspaces.UpdateSettings(ctx, workspaceID, name, timezone, defaultTeamID)
	if err != nil {
		return entity.Workspace{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:   workspaceID,
		WorkspaceName: updated.Name,
		Action:        entity.AuditWorkspaceUpdated,
		ResourceKind:  string(entity.ResourceWorkspace),
		ResourceID:    workspaceID,
	})

	return updated, nil
}

func (s *workspacesService) assignableTeam(ctx context.Context, workspaceID, teamID uuid.UUID) (uuid.UUID, error) {
	rejected := entity.NewValidationError(entity.FieldError{
		Field: "defaultTeamId",
		Code:  entity.ValidationCodeUnsupportedValue,
	})

	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, entity.ErrTeamNotFound) {
			return uuid.Nil, rejected
		}

		return uuid.Nil, err
	}

	if team.WorkspaceID != workspaceID || team.Archived() {
		return uuid.Nil, rejected
	}

	return team.ID, nil
}

func (s *workspacesService) Delete(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionDelete, WorkspaceID: workspaceID}); err != nil {
		return entity.Workspace{}, err
	}

	now := time.Now().UTC()
	purgeAfter := now.Add(s.workspaceCfg.DeletionGracePeriod)

	var deleted entity.Workspace

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
			return err
		}

		current, err := s.workspaces.GetByID(ctx, workspaceID)
		if err != nil {
			return err
		}

		if !current.Status.CanTransitionTo(entity.WorkspaceStatusPendingDeletion) {
			return entity.WorkspaceDeletedError{PurgeAfter: current.PurgeAfter}
		}

		pending, err := s.workspaces.MarkPendingDeletion(ctx, workspaceID, now, purgeAfter)
		if err != nil {
			return err
		}

		deleted = pending

		return nil
	})
	if err != nil {
		return entity.Workspace{}, err
	}

	if err := s.producer.EnqueueWorkspacePurge(ctx, entity.WorkspacePurgePayload{
		WorkspaceID: workspaceID,
	}, purgeAfter); err != nil {
		return entity.Workspace{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:   workspaceID,
		WorkspaceName: deleted.Name,
		Action:        entity.AuditWorkspaceDeletion,
		ResourceKind:  string(entity.ResourceWorkspace),
		ResourceID:    workspaceID,
	})

	return deleted, nil
}

func (s *workspacesService) Restore(ctx context.Context, workspaceID uuid.UUID) (entity.Workspace, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceWorkspace, Action: entity.ActionDelete, WorkspaceID: workspaceID}); err != nil {
		return entity.Workspace{}, err
	}

	restored, err := s.workspaces.Restore(ctx, workspaceID)
	if err != nil {
		return entity.Workspace{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:   workspaceID,
		WorkspaceName: restored.Name,
		Action:        entity.AuditWorkspaceRestored,
		ResourceKind:  string(entity.ResourceWorkspace),
		ResourceID:    workspaceID,
	})

	return restored, nil
}

func (s *workspacesService) Purge(ctx context.Context, workspaceID uuid.UUID) error {
	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, entity.ErrWorkspaceNotFound) {
			return nil
		}

		return err
	}

	if !workspace.PurgeDueAt(time.Now().UTC()) {
		return nil
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.blobs.RemoveAll(ctx, entity.AttachmentPrefix(workspaceID)); err != nil {
			return err
		}

		return s.workspaces.Purge(ctx, workspaceID)
	}); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:   workspaceID,
		WorkspaceName: workspace.Name,
		Action:        entity.AuditWorkspacePurged,
		Actor:         entity.AuditActor{Kind: entity.ActorKindSystem},
		ResourceKind:  string(entity.ResourceWorkspace),
		ResourceID:    workspaceID,
	})

	return nil
}

func (s *workspacesService) ListForAccount(ctx context.Context, accountID uuid.UUID) ([]entity.Workspace, error) {
	actor, ok := identity.From(ctx)
	if !ok || actor != accountID {
		return nil, entity.ErrAccountForbidden
	}

	return s.workspaces.ListByAccountID(ctx, accountID)
}

func (s *workspacesService) ListMembers(ctx context.Context, workspaceID uuid.UUID, input service.ListMembersInput) (service.MemberPage, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceMembership, Action: entity.ActionRead, WorkspaceID: workspaceID}); err != nil {
		return service.MemberPage{}, err
	}

	page := entity.MembershipPage{Query: input.Query, Limit: input.Limit}.Normalized()

	if input.Cursor != "" {
		cursor, err := entity.DecodeMembershipCursor(input.Cursor)
		if err != nil {
			return service.MemberPage{}, entity.NewValidationError(entity.FieldError{
				Field: "cursor",
				Code:  entity.ValidationCodeMalformed,
			})
		}

		page.Cursor = &cursor
	}

	members, err := s.memberships.ListPageByWorkspaceID(ctx, workspaceID, page.Lookahead())
	if err != nil {
		return service.MemberPage{}, err
	}

	if len(members) <= page.Limit {
		return service.MemberPage{Members: members}, nil
	}

	members = members[:page.Limit]

	return service.MemberPage{
		Members:    members,
		NextCursor: members[len(members)-1].Cursor().Encode(),
	}, nil
}

func (s *workspacesService) AddMember(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.WorkspaceMember, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceMembership, Action: entity.ActionManage, WorkspaceID: workspaceID}); err != nil {
		return entity.WorkspaceMember{}, err
	}

	if !role.Valid() {
		return entity.WorkspaceMember{}, entity.NewValidationError(entity.FieldError{
			Field: "role",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return entity.WorkspaceMember{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return entity.WorkspaceMember{}, entity.ErrAccountDeactivated
	}

	created, err := s.memberships.Create(ctx, entity.Membership{
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Role:        role,
		Source:      entity.MembershipSourceManual,
	})
	if err != nil {
		return entity.WorkspaceMember{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditMemberAdded,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   accountID,
		ResourceName: account.DisplayName,
		Detail:       map[string]string{"role": string(role)},
	})

	return describe(created, account), nil
}

func (s *workspacesService) ChangeMemberRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.WorkspaceMember, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceMembership, Action: entity.ActionManage, WorkspaceID: workspaceID}); err != nil {
		return entity.WorkspaceMember{}, err
	}

	actor, ok := identity.From(ctx)
	if !ok {
		return entity.WorkspaceMember{}, entity.ErrAccountForbidden
	}

	if err := guardSelfRoleChange(actor, accountID); err != nil {
		return entity.WorkspaceMember{}, err
	}

	if !role.Valid() {
		return entity.WorkspaceMember{}, entity.NewValidationError(entity.FieldError{
			Field: "role",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	var updated entity.Membership

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		current, err := s.memberships.Get(ctx, workspaceID, accountID)
		if err != nil {
			return err
		}

		if err := guardManualEdit(current); err != nil {
			return err
		}

		if role != entity.MembershipRoleAdmin {
			if err := s.guardLastAdmin(ctx, workspaceID, accountID); err != nil {
				return err
			}
		}

		changed, err := s.memberships.UpdateRole(ctx, workspaceID, accountID, role)
		if err != nil {
			return err
		}

		updated = changed

		return nil
	})
	if err != nil {
		return entity.WorkspaceMember{}, err
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return entity.WorkspaceMember{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditMemberRoleChanged,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   accountID,
		ResourceName: account.DisplayName,
		Detail:       map[string]string{"role": string(role)},
	})

	return describe(updated, account), nil
}

func (s *workspacesService) PreviewMemberRemoval(ctx context.Context, workspaceID, accountID uuid.UUID) (service.MemberRemoval, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceMembership, Action: entity.ActionManage, WorkspaceID: workspaceID}); err != nil {
		return service.MemberRemoval{}, err
	}

	membership, err := s.memberships.Get(ctx, workspaceID, accountID)
	if err != nil {
		return service.MemberRemoval{}, err
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return service.MemberRemoval{}, err
	}

	teams, err := s.teams.ListByWorkspaceMember(ctx, workspaceID, accountID)
	if err != nil {
		return service.MemberRemoval{}, err
	}

	sole, err := s.soleAdmin(ctx, accountID, workspaceID)
	if err != nil {
		return service.MemberRemoval{}, err
	}

	return service.MemberRemoval{
		Member:           describe(membership, account),
		Teams:            teams,
		SoleAdmin:        sole,
		DirectoryManaged: membership.Source.Managed(),
	}, nil
}

func (s *workspacesService) RemoveMember(ctx context.Context, workspaceID, accountID uuid.UUID, reassignTo *uuid.UUID) error {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{Resource: entity.ResourceMembership, Action: entity.ActionManage, WorkspaceID: workspaceID}); err != nil {
		return err
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		membership, err := s.memberships.Get(ctx, workspaceID, accountID)
		if err != nil {
			return err
		}

		if err := guardManualEdit(membership); err != nil {
			return err
		}

		if err := s.guardLastAdmin(ctx, workspaceID, accountID); err != nil {
			return err
		}

		if reassignTo != nil {
			if err := s.guardReassignmentTarget(ctx, workspaceID, accountID, *reassignTo); err != nil {
				return err
			}
		}

		return s.memberships.Delete(ctx, workspaceID, accountID)
	}); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditMemberRemoved,
		ResourceKind: string(entity.ResourceMembership),
		ResourceID:   accountID,
	})

	return nil
}

func (s *workspacesService) guardReassignmentTarget(ctx context.Context, workspaceID, from, to uuid.UUID) error {
	rejected := entity.NewValidationError(entity.FieldError{
		Field: "reassignTo",
		Code:  entity.ValidationCodeUnsupportedValue,
	})

	if to == from {
		return rejected
	}

	if _, err := s.memberships.Get(ctx, workspaceID, to); err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return rejected
		}

		return err
	}

	account, err := s.accounts.GetByID(ctx, to)
	if err != nil {
		return err
	}

	if account.Status != entity.AccountStatusActive {
		return rejected
	}

	return nil
}

func (s *workspacesService) soleAdmin(ctx context.Context, accountID, workspaceID uuid.UUID) (bool, error) {
	soleAdminWorkspaceIDs, err := s.memberships.ListWorkspaceIDsWithoutOtherActiveAdmin(ctx, accountID)
	if err != nil {
		return false, err
	}

	return slices.Contains(soleAdminWorkspaceIDs, workspaceID), nil
}

func (s *workspacesService) guardLastAdmin(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	if err := s.workspaces.LockByIDs(ctx, []uuid.UUID{workspaceID}); err != nil {
		return err
	}

	sole, err := s.soleAdmin(ctx, accountID, workspaceID)
	if err != nil {
		return err
	}

	if sole {
		return entity.LastWorkspaceAdminError{WorkspaceIDs: []uuid.UUID{workspaceID}}
	}

	return nil
}

func guardManualEdit(membership entity.Membership) error {
	if membership.Source.Managed() {
		return entity.ErrMembershipDirectoryManaged
	}

	return nil
}

func guardSelfRoleChange(actor, accountID uuid.UUID) error {
	if actor == accountID {
		return entity.ErrMembershipSelfRoleChange
	}

	return nil
}

func describe(membership entity.Membership, account entity.Account) entity.WorkspaceMember {
	return entity.WorkspaceMember{
		Membership:  membership,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}
}

func (s *workspacesService) AuthPolicy(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceAuthPolicy,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	return s.authPolicies.Get(ctx, workspaceID)
}
