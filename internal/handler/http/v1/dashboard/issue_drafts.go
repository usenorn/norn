package dashboard

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceIssueDrafts(
	ctx context.Context,
	request api.ListWorkspaceIssueDraftsRequestObject,
) (api.ListWorkspaceIssueDraftsResponseObject, error) {
	drafts, err := h.drafts.List(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueDrafts200JSONResponse{Drafts: issueDraftDTOs(drafts)}, nil
}

func (h *handler) SaveWorkspaceIssueDraft(
	ctx context.Context,
	request api.SaveWorkspaceIssueDraftRequestObject,
) (api.SaveWorkspaceIssueDraftResponseObject, error) {
	input := service.SaveIssueDraftInput{
		DescriptionDoc: documentOf(request.Body.DescriptionDoc),
		LabelIDs:       identifiersOf(request.Body.LabelIds),
		AttachmentIDs:  identifiersOf(request.Body.AttachmentIds),
	}

	for target, sent := range map[*uuid.UUID]*uuid.UUID{
		&input.DraftID:           request.Body.DraftId,
		&input.TeamID:            request.Body.TeamId,
		&input.StateID:           request.Body.StateId,
		&input.ProjectID:         request.Body.ProjectId,
		&input.CycleID:           request.Body.CycleId,
		&input.AssigneeAccountID: request.Body.AssigneeId,
		&input.ParentIssueID:     request.Body.ParentIssueId,
	} {
		if sent != nil {
			*target = *sent
		}
	}

	if request.Body.Title != nil {
		input.Title = *request.Body.Title
	}

	if request.Body.Description != nil {
		input.Description = *request.Body.Description
	}

	if request.Body.Priority != nil {
		input.Priority = entity.IssuePriority(*request.Body.Priority)
	}

	if request.Body.Estimate != nil {
		input.Estimate = int(*request.Body.Estimate)
	}

	if request.Body.DueOn != nil {
		input.DueOn = request.Body.DueOn.Format(time.DateOnly)
	}

	draft, err := h.drafts.Save(ctx, request.WorkspaceId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SaveWorkspaceIssueDraft200JSONResponse(issueDraftDTO(draft)), nil
}

func (h *handler) DeleteWorkspaceIssueDraft(
	ctx context.Context,
	request api.DeleteWorkspaceIssueDraftRequestObject,
) (api.DeleteWorkspaceIssueDraftResponseObject, error) {
	if err := h.drafts.Remove(ctx, request.WorkspaceId, request.DraftId); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceIssueDraft204Response{}, nil
}
