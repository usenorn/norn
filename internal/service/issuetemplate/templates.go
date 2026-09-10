package issuetemplate

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type issueTemplatesService struct {
	templates  repository.IssueTemplate
	teams      repository.Team
	authorizer service.Authorizer
}

func New(
	templates repository.IssueTemplate,
	teams repository.Team,
	authorizer service.Authorizer,
) service.IssueTemplates {
	return &issueTemplatesService{templates: templates, teams: teams, authorizer: authorizer}
}

func (s *issueTemplatesService) Save(
	ctx context.Context,
	workspaceID uuid.UUID,
	input service.SaveIssueTemplateInput,
) (entity.IssueTemplate, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return entity.IssueTemplate{}, err
	}

	if input.TeamID != uuid.Nil && !decision.Scope.Covers(input.TeamID) {
		return entity.IssueTemplate{}, entity.ErrTeamNotFound
	}

	if err := entity.NewValidationError(
		entity.ValidateTemplateName("name", input.Name),
		entity.ValidateTemplateFields("requiredFields", input.RequiredFields),
	); err != nil {
		return entity.IssueTemplate{}, err
	}

	body, document, err := entity.Described(input.Body, input.BodyDoc)
	if err != nil {
		return entity.IssueTemplate{}, err
	}

	template := entity.IssueTemplate{
		ID:                 input.TemplateID,
		WorkspaceID:        workspaceID,
		TeamID:             input.TeamID,
		Name:               input.Name,
		Description:        input.Description,
		Title:              input.Title,
		Body:               body,
		BodyDoc:            document,
		RequiredFields:     input.RequiredFields,
		StateID:            input.StateID,
		ProjectID:          input.ProjectID,
		AssigneeAccountID:  input.AssigneeAccountID,
		LabelIDs:           input.LabelIDs,
		Priority:           input.Priority,
		Estimate:           input.Estimate,
		Position:           input.Position,
		CreatedByAccountID: decision.Actor.AccountID,
		UpdatedAt:          time.Now().UTC(),
	}

	if template.ID != uuid.Nil {
		return s.templates.Update(ctx, template)
	}

	counted, err := s.templates.CountForTeam(ctx, workspaceID, input.TeamID)
	if err != nil {
		return entity.IssueTemplate{}, err
	}

	if counted >= entity.IssueTemplateMaxPerTeam {
		return entity.IssueTemplate{}, entity.ErrTooManyIssueTemplates
	}

	return s.templates.Create(ctx, template)
}

func (s *issueTemplatesService) ListForTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) ([]entity.IssueTemplate, error) {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return nil, err
	}

	if teamID != uuid.Nil && !decision.Scope.Covers(teamID) {
		return nil, entity.ErrTeamNotFound
	}

	return s.templates.ListForTeam(ctx, workspaceID, teamID)
}

func (s *issueTemplatesService) Remove(
	ctx context.Context,
	workspaceID, templateID uuid.UUID,
) error {
	decision, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceTeam,
		Action:      entity.ActionManage,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
	if err != nil {
		return err
	}

	template, err := s.templates.GetByID(ctx, workspaceID, templateID)
	if err != nil {
		return err
	}

	if template.TeamID != uuid.Nil && !decision.Scope.Covers(template.TeamID) {
		return entity.ErrIssueTemplateNotFound
	}

	return s.templates.Remove(ctx, workspaceID, templateID)
}
