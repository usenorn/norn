package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueTemplates(
	ctx context.Context,
	request api.ListWorkspaceIssueTemplatesRequestObject,
) (api.ListWorkspaceIssueTemplatesResponseObject, error) {
	teamID := uuid.Nil
	if request.Params.TeamId != nil {
		teamID = *request.Params.TeamId
	}

	templates, err := h.templates.ListForTeam(ctx, request.WorkspaceId, teamID)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueTemplates200JSONResponse{
		Templates: issueTemplateDTOs(templates),
	}, nil
}

func (h *handler) SaveWorkspaceIssueTemplate(
	ctx context.Context,
	request api.SaveWorkspaceIssueTemplateRequestObject,
) (api.SaveWorkspaceIssueTemplateResponseObject, error) {
	input := service.SaveIssueTemplateInput{
		Name:           request.Body.Name,
		BodyDoc:        documentOf(request.Body.BodyDoc),
		RequiredFields: templateFieldsOf(request.Body.RequiredFields),
		LabelIDs:       identifiersOf(request.Body.LabelIds),
	}

	for target, sent := range map[*uuid.UUID]*uuid.UUID{
		&input.TemplateID:        request.Body.TemplateId,
		&input.TeamID:            request.Body.TeamId,
		&input.StateID:           request.Body.StateId,
		&input.ProjectID:         request.Body.ProjectId,
		&input.AssigneeAccountID: request.Body.AssigneeId,
	} {
		if sent != nil {
			*target = *sent
		}
	}

	for target, sent := range map[*string]*string{
		&input.Description: request.Body.Description,
		&input.Title:       request.Body.Title,
		&input.Body:        request.Body.Body,
	} {
		if sent != nil {
			*target = *sent
		}
	}

	if request.Body.Priority != nil {
		input.Priority = entity.IssuePriority(*request.Body.Priority)
	}

	if request.Body.Estimate != nil {
		input.Estimate = int(*request.Body.Estimate)
	}

	if request.Body.Position != nil {
		input.Position = int(*request.Body.Position)
	}

	template, err := h.templates.Save(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SaveWorkspaceIssueTemplate200JSONResponse(issueTemplateDTO(template)), nil
}

func (h *handler) DeleteWorkspaceIssueTemplate(
	ctx context.Context,
	request api.DeleteWorkspaceIssueTemplateRequestObject,
) (api.DeleteWorkspaceIssueTemplateResponseObject, error) {
	if err := h.templates.Remove(ctx, request.WorkspaceId, request.TemplateId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceIssueTemplate204Response{}, nil
}

func templateFieldsOf(fields *[]api.TemplateField) []entity.TemplateField {
	if fields == nil {
		return nil
	}

	named := make([]entity.TemplateField, 0, len(*fields))

	for _, field := range *fields {
		named = append(named, entity.TemplateField(field))
	}

	return named
}
