package dashboard

import (
	"context"

	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) GetWorkspaceReviewQueue(
	ctx context.Context,
	request api.GetWorkspaceReviewQueueRequestObject,
) (api.GetWorkspaceReviewQueueResponseObject, error) {
	queue, err := h.executions.Queue(ctx, request.WorkspaceId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dto := api.GetWorkspaceReviewQueue200JSONResponse{
		Questions: make([]api.ReviewQuestion, 0, len(queue.Questions)),
		Plans:     reviewRunDTOs(queue.Plans),
		Changes:   reviewRunDTOs(queue.Changes),
	}

	for _, waiting := range queue.Questions {
		dto.Questions = append(dto.Questions, api.ReviewQuestion{
			Question: issueQuestionDTO(waiting.Question),
			Decision: decisionRightDTO(waiting.Right),
		})
	}

	return dto, nil
}

func (h *handler) GetWorkspaceIssueDecisionRight(
	ctx context.Context,
	request api.GetWorkspaceIssueDecisionRightRequestObject,
) (api.GetWorkspaceIssueDecisionRightResponseObject, error) {
	right, err := h.executions.DecisionRight(ctx, request.WorkspaceId, request.IssueId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.GetWorkspaceIssueDecisionRight200JSONResponse(decisionRightDTO(right)), nil
}

func reviewRunDTOs(runs []service.ReviewRun) []api.ReviewRun {
	dtos := make([]api.ReviewRun, 0, len(runs))

	for _, waiting := range runs {
		dtos = append(dtos, api.ReviewRun{
			Run:      executionSummaryDTO(waiting.Listing),
			Decision: decisionRightDTO(waiting.Right),
		})
	}

	return dtos
}

func decisionRightDTO(right service.DecisionRight) api.DecisionRight {
	return api.DecisionRight{CanDecide: right.CanDecide, Decider: nilIfEmpty(right.MakerName)}
}
