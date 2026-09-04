package team

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type teamsService struct {
	teams        repository.Team
	teamMembers  repository.TeamMember
	workspaces   repository.Workspace
	memberships  repository.Membership
	accounts     repository.Account
	authPolicies repository.WorkspaceAuthPolicy
	states       repository.WorkflowState
	rules        repository.SCMTransitionRule
	notify       repository.NotificationEvent
	authorizer   service.Authorizer
	transactor   repository.Transactor
	audit        service.Audit
	events       service.Events
	emitter      service.WebhookEmitter
}

func New(
	teams repository.Team,
	teamMembers repository.TeamMember,
	workspaces repository.Workspace,
	memberships repository.Membership,
	accounts repository.Account,
	authPolicies repository.WorkspaceAuthPolicy,
	states repository.WorkflowState,
	rules repository.SCMTransitionRule,
	notify repository.NotificationEvent,
	authorizer service.Authorizer,
	transactor repository.Transactor,
	audit service.Audit,
	events service.Events,
	emitter service.WebhookEmitter,
) service.Teams {
	return &teamsService{
		teams:        teams,
		teamMembers:  teamMembers,
		workspaces:   workspaces,
		memberships:  memberships,
		accounts:     accounts,
		authPolicies: authPolicies,
		states:       states,
		rules:        rules,
		notify:       notify,
		authorizer:   authorizer,
		transactor:   transactor,
		audit:        audit,
		events:       events,
		emitter:      emitter,
	}
}

func (s *teamsService) Create(ctx context.Context, input service.CreateTeamInput) (entity.Team, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: input.WorkspaceID,
	}); err != nil {
		return entity.Team{}, err
	}

	key := entity.NormalizeTeamKey(input.Key)

	if err := entity.NewValidationError(
		entity.ValidateTeamKey("key", key),
		entity.ValidateTeamName("name", input.Name),
	); err != nil {
		return entity.Team{}, err
	}

	visibility := input.Visibility
	if visibility == "" {
		visibility = entity.DefaultTeamVisibility
	}

	if !visibility.Valid() {
		return entity.Team{}, entity.NewValidationError(entity.FieldError{
			Field: "visibility",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	var created entity.Team

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		team, err := s.teams.Create(ctx, entity.Team{
			WorkspaceID: input.WorkspaceID,
			Key:         key,
			Name:        input.Name,
			Status:      entity.TeamStatusActive,
			Visibility:  visibility,
		})
		if err != nil {
			return err
		}

		states, err := s.states.CreateMany(
			ctx,
			entity.DefaultWorkflowStates(input.WorkspaceID, team.ID),
		)
		if err != nil {
			return err
		}

		if err := s.rules.CreateMany(
			ctx,
			entity.DefaultSCMTransitionRules(input.WorkspaceID, team.ID, states),
		); err != nil {
			return err
		}

		created = team

		return nil
	})
	if err != nil {
		return entity.Team{}, err
	}

	return created, nil
}

func (s *teamsService) Get(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Team{}, err
	}

	return s.scopedTeam(ctx, workspaceID, teamID, decision)
}

func (s *teamsService) List(ctx context.Context, workspaceID uuid.UUID, status entity.TeamStatus) ([]entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	if status != "" && !status.Valid() {
		return nil, entity.NewValidationError(entity.FieldError{
			Field: "status",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	teams, err := s.teams.ListVisibleTo(
		ctx,
		workspaceID,
		decision.Actor.AccountID,
		status,
		decision.Scope.IncludePrivate,
	)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(teams, func(team entity.Team) bool {
		return !decision.Scope.Covers(team.ID)
	}), nil
}

func (s *teamsService) Update(ctx context.Context, workspaceID, teamID uuid.UUID, input service.UpdateTeamInput) (entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Team{}, err
	}

	current, err := s.liveTeam(ctx, workspaceID, teamID, decision)
	if err != nil {
		return entity.Team{}, err
	}

	settings := repository.TeamSettings{
		Name:        current.Name,
		Description: current.Description,
		Icon:        current.Icon,
		IconColor:   current.IconColor,
		Estimation:  current.Estimation,
		Visibility:  current.Visibility,
	}

	if input.Name != nil {
		settings.Name = *input.Name
	}

	if input.Description != nil {
		settings.Description = *input.Description
	}

	if input.Icon != nil {
		settings.Icon = *input.Icon
	}

	if input.IconColor != nil {
		settings.IconColor = *input.IconColor
	}

	if input.Estimation != nil {
		settings.Estimation = *input.Estimation
	}

	if input.Visibility != nil {
		settings.Visibility = *input.Visibility
	}

	if err := entity.NewValidationError(
		entity.ValidateTeamName("name", settings.Name),
		entity.ValidateTeamDescription("description", settings.Description),
		entity.ValidateTeamIcon("icon", settings.Icon),
		entity.ValidateTeamColor("iconColor", settings.IconColor),
		entity.ValidateTeamEstimation("estimation", settings.Estimation),
	); err != nil {
		return entity.Team{}, err
	}

	if !settings.Visibility.Valid() {
		return entity.Team{}, entity.NewValidationError(entity.FieldError{
			Field: "visibility",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	return s.teams.UpdateSettings(ctx, teamID, settings)
}

func (s *teamsService) Archive(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Team{}, err
	}

	if _, err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return entity.Team{}, err
	}

	return s.teams.Archive(ctx, teamID, time.Now().UTC())
}

func (s *teamsService) Unarchive(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.Team, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.Team{}, err
	}

	if _, err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return entity.Team{}, err
	}

	return s.teams.Unarchive(ctx, teamID)
}

func (s *teamsService) ListMembers(ctx context.Context, workspaceID, teamID uuid.UUID) ([]service.TeamMemberView, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeamMembership,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.scopedTeam(ctx, workspaceID, teamID, decision); err != nil {
		return nil, err
	}

	memberships, err := s.teamMembers.ListByTeamID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	return s.describe(ctx, memberships)
}

func (s *teamsService) AddMember(ctx context.Context, workspaceID, teamID, accountID uuid.UUID) (service.TeamMemberView, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeamMembership,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return service.TeamMemberView{}, err
	}

	team, err := s.liveTeam(ctx, workspaceID, teamID, decision)
	if err != nil {
		return service.TeamMemberView{}, err
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return service.TeamMemberView{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return service.TeamMemberView{}, entity.ErrAccountDeactivated
	}

	var created entity.TeamMembership

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		created, err = s.teamMembers.Create(ctx, entity.TeamMembership{
			WorkspaceID: workspaceID,
			TeamID:      teamID,
			AccountID:   accountID,
		})
		if err != nil {
			return err
		}

		attribution := decision.ActivityActor()

		if err := s.notify.Record(ctx, entity.NotificationEvent{
			WorkspaceID: workspaceID,
			Subject:     entity.NotifyTeam(teamID),
			Kind:        entity.NotificationKindMembership,
			Actor:       attribution.AccountID,
			ActorKind:   attribution.Kind,
			Target:      accountID,
		}); err != nil {
			return err
		}

		s.rescope(ctx, workspaceID, accountID)

		return s.emit(ctx, entity.WebhookTeamMembershipAdded, accountID, team, decision)
	}); err != nil {
		return service.TeamMemberView{}, err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditTeamMemberAdded,
		ResourceKind: string(entity.ResourceTeamMembership),
		ResourceID:   accountID,
		ResourceName: account.DisplayName,
		Detail:       map[string]string{"team_id": teamID.String()},
	})

	return service.TeamMemberView{
		Membership:  created,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, nil
}

func (s *teamsService) RemoveMember(ctx context.Context, workspaceID, teamID, accountID uuid.UUID) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeamMembership,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return err
	}

	team, err := s.liveTeam(ctx, workspaceID, teamID, decision)
	if err != nil {
		return err
	}

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.teamMembers.Delete(ctx, teamID, accountID); err != nil {
			return err
		}

		s.rescope(ctx, workspaceID, accountID)

		return s.emit(ctx, entity.WebhookTeamMembershipRemoved, accountID, team, decision)
	}); err != nil {
		return err
	}

	s.audit.Record(ctx, entity.AuditEntry{
		WorkspaceID:  workspaceID,
		Action:       entity.AuditTeamMemberRemoved,
		ResourceKind: string(entity.ResourceTeamMembership),
		ResourceID:   accountID,
		Detail:       map[string]string{"team_id": teamID.String()},
	})

	return nil
}

func (s *teamsService) describe(ctx context.Context, memberships []entity.TeamMembership) ([]service.TeamMemberView, error) {
	if len(memberships) == 0 {
		return []service.TeamMemberView{}, nil
	}

	ids := make([]uuid.UUID, len(memberships))
	for i, membership := range memberships {
		ids[i] = membership.AccountID
	}

	accounts, err := s.accounts.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]entity.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}

	views := make([]service.TeamMemberView, 0, len(memberships))

	for _, membership := range memberships {
		account := byID[membership.AccountID]

		views = append(views, service.TeamMemberView{
			Membership:  membership,
			DisplayName: account.DisplayName,
			Email:       account.Email,
		})
	}

	return views, nil
}

func (s *teamsService) scopedTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (entity.Team, error) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.Team{}, err
	}

	if team.WorkspaceID != workspaceID {
		return entity.Team{}, entity.ErrTeamNotFound
	}

	if err := s.visible(decision, team); err != nil {
		return entity.Team{}, err
	}

	return team, nil
}

func (s *teamsService) liveTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
) (entity.Team, error) {
	team, err := s.scopedTeam(ctx, workspaceID, teamID, decision)
	if err != nil {
		return entity.Team{}, err
	}

	if team.Archived() {
		return entity.Team{}, entity.ErrTeamArchived
	}

	return team, nil
}

func (s *teamsService) visible(decision entity.Decision, team entity.Team) error {
	if !decision.Scope.Covers(team.ID) {
		return entity.ErrTeamNotFound
	}

	return nil
}
