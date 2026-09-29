package dashboard

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func (h *handler) ListWorkspaceExecutionPlans(
	ctx context.Context,
	request api.ListWorkspaceExecutionPlansRequestObject,
) (api.ListWorkspaceExecutionPlansResponseObject, error) {
	plans, err := h.executions.Plans(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dtos := make([]api.ExecutionPlan, 0, len(plans))
	for _, plan := range plans {
		dtos = append(dtos, executionPlanDTO(plan))
	}

	return api.ListWorkspaceExecutionPlans200JSONResponse(dtos), nil
}

func (h *handler) ApproveWorkspaceExecutionPlan(
	ctx context.Context,
	request api.ApproveWorkspaceExecutionPlanRequestObject,
) (api.ApproveWorkspaceExecutionPlanResponseObject, error) {
	execution, err := h.executions.ApprovePlan(
		ctx, request.WorkspaceId, request.ExecutionId, request.Revision,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ApproveWorkspaceExecutionPlan200JSONResponse(executionDTO(execution)), nil
}

func (h *handler) ReviseWorkspaceExecutionPlan(
	ctx context.Context,
	request api.ReviseWorkspaceExecutionPlanRequestObject,
) (api.ReviseWorkspaceExecutionPlanResponseObject, error) {
	execution, err := h.executions.RevisePlan(
		ctx, request.WorkspaceId, request.ExecutionId, request.Revision, request.Body.Feedback,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReviseWorkspaceExecutionPlan200JSONResponse(executionDTO(execution)), nil
}

func (h *handler) GetWorkspaceExecutionReview(
	ctx context.Context,
	request api.GetWorkspaceExecutionReviewRequestObject,
) (api.GetWorkspaceExecutionReviewResponseObject, error) {
	state, err := h.executions.Review(ctx, request.WorkspaceId, request.ExecutionId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	viewer, _ := h.currentAccountID(ctx)

	comments := make([]api.ReviewComment, 0, len(state.Comments))
	for _, comment := range state.Comments {
		comments = append(comments, reviewCommentDTO(comment, state.Heads, viewer))
	}

	reviews := make([]api.ExecutionReview, 0, len(state.Reviews))
	for _, review := range state.Reviews {
		reviews = append(reviews, executionReviewDTO(review))
	}

	return api.GetWorkspaceExecutionReview200JSONResponse(api.ExecutionReviewState{
		Heads:    reviewHeadDTOs(state.Heads),
		Comments: comments,
		Reviews:  reviews,
	}), nil
}

func (h *handler) CommentOnWorkspaceExecutionReview(
	ctx context.Context,
	request api.CommentOnWorkspaceExecutionReviewRequestObject,
) (api.CommentOnWorkspaceExecutionReviewResponseObject, error) {
	body := request.Body

	draft := service.ReviewCommentDraft{
		Anchor: entity.ReviewAnchor{
			Repository: textOf(body.Repository),
			Path:       textOf(body.Path),
			Hunk:       textOf(body.Hunk),
		},
		Body:    body.Body,
		Publish: body.Publish != nil && *body.Publish,
	}

	if body.ParentId != nil {
		draft.ParentID = *body.ParentId
	}

	if body.Side != nil {
		draft.Anchor.Side = entity.ReviewSide(*body.Side)
	}

	if body.Line != nil {
		draft.Anchor.Line = *body.Line
	}

	comment, err := h.executions.CommentOnReview(ctx, request.WorkspaceId, request.ExecutionId, draft)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dto, err := h.reviewCommentFor(ctx, request.WorkspaceId, request.ExecutionId, comment)
	if err != nil {
		return nil, err
	}

	return api.CommentOnWorkspaceExecutionReview201JSONResponse(dto), nil
}

func (h *handler) EditWorkspaceExecutionReviewComment(
	ctx context.Context,
	request api.EditWorkspaceExecutionReviewCommentRequestObject,
) (api.EditWorkspaceExecutionReviewCommentResponseObject, error) {
	comment, err := h.executions.EditReviewComment(
		ctx, request.WorkspaceId, request.ExecutionId, request.ReviewCommentId, request.Body.Body,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dto, err := h.reviewCommentFor(ctx, request.WorkspaceId, request.ExecutionId, comment)
	if err != nil {
		return nil, err
	}

	return api.EditWorkspaceExecutionReviewComment200JSONResponse(dto), nil
}

func (h *handler) DeleteWorkspaceExecutionReviewComment(
	ctx context.Context,
	request api.DeleteWorkspaceExecutionReviewCommentRequestObject,
) (api.DeleteWorkspaceExecutionReviewCommentResponseObject, error) {
	if err := h.executions.DeleteReviewComment(
		ctx, request.WorkspaceId, request.ExecutionId, request.ReviewCommentId,
	); err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.DeleteWorkspaceExecutionReviewComment204Response{}, nil
}

func (h *handler) ResolveWorkspaceExecutionReviewComment(
	ctx context.Context,
	request api.ResolveWorkspaceExecutionReviewCommentRequestObject,
) (api.ResolveWorkspaceExecutionReviewCommentResponseObject, error) {
	comment, err := h.executions.ResolveReviewComment(
		ctx, request.WorkspaceId, request.ExecutionId, request.ReviewCommentId, request.Body.Resolved,
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	dto, err := h.reviewCommentFor(ctx, request.WorkspaceId, request.ExecutionId, comment)
	if err != nil {
		return nil, err
	}

	return api.ResolveWorkspaceExecutionReviewComment200JSONResponse(dto), nil
}

func (h *handler) SubmitWorkspaceExecutionReview(
	ctx context.Context,
	request api.SubmitWorkspaceExecutionReviewRequestObject,
) (api.SubmitWorkspaceExecutionReviewResponseObject, error) {
	heads := make(entity.ReviewHeads, len(request.Body.Heads))
	for _, head := range request.Body.Heads {
		heads[head.Repository] = head.HeadSha
	}

	review, err := h.executions.SubmitReview(
		ctx, request.WorkspaceId, request.ExecutionId, service.ReviewSubmission{
			Verdict: entity.ExecutionReviewVerdict(request.Body.Verdict),
			Summary: textOf(request.Body.Summary),
			Heads:   heads,
		},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.SubmitWorkspaceExecutionReview201JSONResponse(executionReviewDTO(review)), nil
}

func (h *handler) reviewCommentFor(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	comment entity.ExecutionReviewComment,
) (api.ReviewComment, error) {
	viewer, _ := h.currentAccountID(ctx)

	state, err := h.executions.Review(ctx, workspaceID, executionID)
	if err != nil {
		return api.ReviewComment{}, err
	}

	return reviewCommentDTO(comment, state.Heads, viewer), nil
}

func executionPlanDTO(plan entity.ExecutionPlan) api.ExecutionPlan {
	return api.ExecutionPlan{
		Id:                      plan.ID,
		ExecutionId:             plan.ExecutionID,
		Revision:                plan.Revision,
		Body:                    plan.Body,
		ProposedAt:              plan.ProposedAt,
		ApprovedByName:          nilIfEmpty(plan.ApprovedByName),
		ApprovedAt:              plan.ApprovedAt,
		RevisionFeedback:        nilIfEmpty(plan.RevisionFeedback),
		RevisionRequestedByName: nilIfEmpty(plan.RevisionRequestedByName),
		RevisionRequestedAt:     plan.RevisionRequestedAt,
	}
}

func reviewCommentDTO(
	comment entity.ExecutionReviewComment,
	heads entity.ReviewHeads,
	viewer uuid.UUID,
) api.ReviewComment {
	mine := viewer != uuid.Nil && comment.AuthorAccountID == viewer

	return api.ReviewComment{
		Id:             comment.ID,
		ExecutionId:    comment.ExecutionID,
		ReviewId:       nilIfNilID(comment.ReviewID),
		ParentId:       nilIfNilID(comment.ParentID),
		Pending:        comment.Pending(),
		Repository:     comment.Anchor.Repository,
		Path:           comment.Anchor.Path,
		Side:           api.ReviewSide(comment.Anchor.Side),
		Line:           comment.Anchor.Line,
		HeadSha:        comment.Anchor.HeadSHA,
		Outdated:       comment.Outdated(heads),
		Hunk:           nilIfEmpty(comment.Anchor.Hunk),
		Body:           comment.Body,
		AuthorName:     nilIfEmpty(comment.AuthorName),
		Mine:           &mine,
		CreatedAt:      comment.CreatedAt,
		EditedAt:       comment.EditedAt,
		ResolvedAt:     comment.ResolvedAt,
		ResolvedByName: nilIfEmpty(comment.ResolvedByName),
	}
}

func executionReviewDTO(review entity.ExecutionReview) api.ExecutionReview {
	return api.ExecutionReview{
		Id:          review.ID,
		ExecutionId: review.ExecutionID,
		Verdict:     api.ExecutionReviewVerdict(review.Verdict),
		Summary:     review.Summary,
		AuthorName:  nilIfEmpty(review.AuthorName),
		SubmittedAt: review.SubmittedAt,
	}
}

func reviewHeadDTOs(heads entity.ReviewHeads) []api.ReviewHead {
	dtos := make([]api.ReviewHead, 0, len(heads))

	for repository, head := range heads {
		dtos = append(dtos, api.ReviewHead{Repository: repository, HeadSha: head})
	}

	sort.Slice(dtos, func(i, j int) bool { return dtos[i].Repository < dtos[j].Repository })

	return dtos
}
