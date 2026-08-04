package project

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type projectsService struct {
	projects    repository.Project
	members     repository.ProjectMember
	statuses    repository.ProjectStatusUpdate
	accounts    repository.Account
	memberships repository.Membership
	authorizer  service.Authorizer
	transactor  repository.Transactor
}

func New(
	projects repository.Project,
	members repository.ProjectMember,
	statuses repository.ProjectStatusUpdate,
	accounts repository.Account,
	memberships repository.Membership,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.Projects {
	return &projectsService{
		projects:    projects,
		members:     members,
		statuses:    statuses,
		accounts:    accounts,
		memberships: memberships,
		authorizer:  authorizer,
		transactor:  transactor,
	}
}

func (s *projectsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceProject,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func editable(project entity.Project, decision entity.Decision) error {
	if decision.Role == entity.MembershipRoleAdmin || project.LedBy(decision.Actor.AccountID) {
		return nil
	}

	return entity.ErrProjectNotLead
}

func (s *projectsService) view(
	ctx context.Context,
	project entity.Project,
	scope entity.TeamScope,
) (service.ProjectView, error) {
	concealed, err := s.projects.HasConcealedWork(ctx, scope, project.ID)
	if err != nil {
		return service.ProjectView{}, err
	}

	return service.ProjectView{Project: project, ConcealedWork: concealed}, nil
}

func (s *projectsService) Create(
	ctx context.Context,
	input service.CreateProjectInput,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, input.WorkspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectView{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateProjectName("name", input.Name),
		entity.ValidateProjectSlug("slug", input.Slug),
		entity.ValidateProjectDescription("description", input.Description),
		entity.ValidateTargetDate("targetOn", input.TargetOn),
	); err != nil {
		return service.ProjectView{}, err
	}

	if err := s.leadable(ctx, input.WorkspaceID, input.LeadAccountID); err != nil {
		return service.ProjectView{}, err
	}

	project := entity.Project{
		WorkspaceID: input.WorkspaceID,
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		State:       entity.ProjectStatePlanned,
		TargetOn:    input.TargetOn,
	}

	if input.LeadAccountID != nil {
		project.LeadAccountID = *input.LeadAccountID
	}

	created, err := s.projects.Create(ctx, project)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, created, decision.Scope)
}

func (s *projectsService) leadable(
	ctx context.Context,
	workspaceID uuid.UUID,
	accountID *uuid.UUID,
) error {
	if accountID == nil {
		return nil
	}

	if _, err := s.memberships.Get(ctx, workspaceID, *accountID); err != nil {
		if errors.Is(err, entity.ErrMembershipNotFound) {
			return entity.NewValidationError(entity.FieldError{
				Field: "leadAccountId",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}

		return err
	}

	return nil
}

func (s *projectsService) Get(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.ProjectView{}, err
	}

	project, err := s.projects.GetByID(ctx, workspaceID, projectID)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, project, decision.Scope)
}

func (s *projectsService) GetBySlug(
	ctx context.Context,
	workspaceID uuid.UUID,
	slug string,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return service.ProjectView{}, err
	}

	project, err := s.projects.GetBySlug(ctx, workspaceID, slug)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, project, decision.Scope)
}

func (s *projectsService) List(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.ListProjectsInput,
) ([]service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	filter := repository.ProjectFilter{State: input.State, IncludeArchived: input.Archived}

	if input.Mine {
		actor := decision.Actor.AccountID
		filter.ForAccountID = &actor
	}

	projects, err := s.projects.ListByWorkspaceID(ctx, workspaceID, filter)
	if err != nil {
		return nil, err
	}

	views := make([]service.ProjectView, 0, len(projects))

	for _, project := range projects {
		views = append(views, service.ProjectView{Project: project})
	}

	return views, nil
}

func (s *projectsService) writable(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
	decision entity.Decision,
) (entity.Project, error) {
	project, err := s.projects.GetByID(ctx, workspaceID, projectID)
	if err != nil {
		return entity.Project{}, err
	}

	if err := editable(project, decision); err != nil {
		return entity.Project{}, err
	}

	if project.Archived() {
		return entity.Project{}, entity.ErrProjectArchived
	}

	return project, nil
}

func (s *projectsService) Update(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
	input service.UpdateProjectInput,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectView{}, err
	}

	if _, err := s.writable(ctx, workspaceID, projectID, decision); err != nil {
		return service.ProjectView{}, err
	}

	fields := make([]entity.FieldError, 0, 3)

	if input.Name != nil {
		fields = append(fields, entity.ValidateProjectName("name", *input.Name))
	}

	if input.Description != nil {
		fields = append(fields, entity.ValidateProjectDescription("description", *input.Description))
	}

	if input.TargetOn != nil {
		fields = append(fields, entity.ValidateTargetDate("targetOn", *input.TargetOn))
	}

	for _, field := range input.Clear {
		if !slices.Contains(clearable, field) {
			fields = append(fields, entity.FieldError{
				Field: "clear",
				Code:  entity.ValidationCodeUnsupportedValue,
			})
		}
	}

	if err := entity.NewValidationError(fields...); err != nil {
		return service.ProjectView{}, err
	}

	if err := s.leadable(ctx, workspaceID, input.LeadAccountID); err != nil {
		return service.ProjectView{}, err
	}

	settings := repository.ProjectSettings{
		LeadAccountID: input.LeadAccountID,
		ClearLead:     slices.Contains(input.Clear, projectFieldLead),
		ClearTarget:   slices.Contains(input.Clear, projectFieldTarget),
	}

	if input.Name != nil {
		settings.Name = *input.Name
	}

	if input.Description != nil {
		settings.Description = *input.Description
	}

	if input.TargetOn != nil {
		settings.TargetOn = *input.TargetOn
	}

	updated, err := s.projects.UpdateSettings(ctx, projectID, settings)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, updated, decision.Scope)
}

const (
	projectFieldLead   = "lead"
	projectFieldTarget = "targetOn"
)

var clearable = []string{projectFieldLead, projectFieldTarget}

func (s *projectsService) SetState(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
	state entity.ProjectState,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectView{}, err
	}

	if !state.Valid() {
		return service.ProjectView{}, entity.NewValidationError(entity.FieldError{
			Field: "state",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if _, err := s.writable(ctx, workspaceID, projectID, decision); err != nil {
		return service.ProjectView{}, err
	}

	updated, err := s.projects.SetState(ctx, projectID, state)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, updated, decision.Scope)
}

func (s *projectsService) Archive(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectView{}, err
	}

	project, err := s.projects.GetByID(ctx, workspaceID, projectID)
	if err != nil {
		return service.ProjectView{}, err
	}

	if err := editable(project, decision); err != nil {
		return service.ProjectView{}, err
	}

	if !project.State.Finished() {
		return service.ProjectView{}, entity.ErrProjectNotFinished
	}

	archived, err := s.projects.Archive(ctx, projectID, time.Now().UTC())
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, archived, decision.Scope)
}

func (s *projectsService) Unarchive(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) (service.ProjectView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectView{}, err
	}

	project, err := s.projects.GetByID(ctx, workspaceID, projectID)
	if err != nil {
		return service.ProjectView{}, err
	}

	if err := editable(project, decision); err != nil {
		return service.ProjectView{}, err
	}

	restored, err := s.projects.Unarchive(ctx, projectID)
	if err != nil {
		return service.ProjectView{}, err
	}

	return s.view(ctx, restored, decision.Scope)
}

func (s *projectsService) Remove(ctx context.Context, workspaceID, projectID uuid.UUID) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if decision.Role != entity.MembershipRoleAdmin {
		return entity.ErrProjectNotLead
	}

	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return err
	}

	return s.projects.Delete(ctx, projectID)
}

func (s *projectsService) ListMembers(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) ([]service.ProjectMemberView, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}

	memberships, err := s.members.ListByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return s.describe(ctx, memberships)
}

func (s *projectsService) describe(
	ctx context.Context,
	memberships []entity.ProjectMembership,
) ([]service.ProjectMemberView, error) {
	views := make([]service.ProjectMemberView, 0, len(memberships))

	if len(memberships) == 0 {
		return views, nil
	}

	ids := make([]uuid.UUID, 0, len(memberships))
	for _, membership := range memberships {
		ids = append(ids, membership.AccountID)
	}

	accounts, err := s.accounts.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]entity.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}

	for _, membership := range memberships {
		account := byID[membership.AccountID]
		views = append(views, service.ProjectMemberView{
			Membership:  membership,
			DisplayName: account.DisplayName,
			Email:       account.Email,
		})
	}

	return views, nil
}

func (s *projectsService) AddMember(
	ctx context.Context,
	workspaceID, projectID, accountID uuid.UUID,
) (service.ProjectMemberView, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return service.ProjectMemberView{}, err
	}

	if _, err := s.writable(ctx, workspaceID, projectID, decision); err != nil {
		return service.ProjectMemberView{}, err
	}

	account, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return service.ProjectMemberView{}, err
	}

	if account.Status != entity.AccountStatusActive {
		return service.ProjectMemberView{}, entity.ErrAccountDeactivated
	}

	created, err := s.members.Create(ctx, entity.ProjectMembership{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		AccountID:   accountID,
	})
	if err != nil {
		return service.ProjectMemberView{}, err
	}

	return service.ProjectMemberView{
		Membership:  created,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, nil
}

func (s *projectsService) RemoveMember(
	ctx context.Context,
	workspaceID, projectID, accountID uuid.UUID,
) error {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return err
	}

	if _, err := s.writable(ctx, workspaceID, projectID, decision); err != nil {
		return err
	}

	return s.members.Delete(ctx, projectID, accountID)
}

func (s *projectsService) PostStatus(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
	input service.PostProjectStatusInput,
) (entity.ProjectStatusUpdate, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.ProjectStatusUpdate{}, err
	}

	if !input.Health.Valid() {
		return entity.ProjectStatusUpdate{}, entity.NewValidationError(entity.FieldError{
			Field: "health",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if err := entity.NewValidationError(
		entity.ValidateProjectStatusBody("body", input.Body),
	); err != nil {
		return entity.ProjectStatusUpdate{}, err
	}

	if _, err := s.writable(ctx, workspaceID, projectID, decision); err != nil {
		return entity.ProjectStatusUpdate{}, err
	}

	return s.statuses.Record(ctx, entity.ProjectStatusUpdate{
		ProjectID:       projectID,
		AuthorAccountID: decision.Actor.AccountID,
		Health:          input.Health,
		Body:            input.Body,
	})
}

func (s *projectsService) ListStatus(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) ([]entity.ProjectStatusUpdate, error) {
	if _, err := s.decide(ctx, workspaceID, entity.ActionRead); err != nil {
		return nil, err
	}

	if _, err := s.projects.GetByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}

	return s.statuses.ListByProjectID(ctx, projectID)
}
