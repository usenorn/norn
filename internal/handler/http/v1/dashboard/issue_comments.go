package dashboard

import (
	"context"

	api "github.com/usenorn/norn/pkg/http/v1/dashboard"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

func (h *handler) ListWorkspaceIssueComments(
	ctx context.Context,
	request api.ListWorkspaceIssueCommentsRequestObject,
) (api.ListWorkspaceIssueCommentsResponseObject, error) {
	input := service.ListCommentsInput{}

	if request.Params.Limit != nil {
		input.Limit = int(*request.Params.Limit)
	}

	if request.Params.Cursor != nil {
		input.Cursor = *request.Params.Cursor
	}

	thread, err := h.issueComments.List(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ListWorkspaceIssueComments200JSONResponse(commentPageDTO(thread)), nil
}

func (h *handler) PostWorkspaceIssueComment(
	ctx context.Context,
	request api.PostWorkspaceIssueCommentRequestObject,
) (api.PostWorkspaceIssueCommentResponseObject, error) {
	input := service.PostCommentInput{Body: request.Body.Body}

	if request.Body.ParentCommentId != nil {
		input.ParentCommentID = *request.Body.ParentCommentId
	}

	if request.Body.Mentions != nil {
		input.Mentions = mentionInputs(*request.Body.Mentions)
	}

	posted, err := h.issueComments.Post(ctx, request.WorkspaceId, request.IssueId, input)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.PostWorkspaceIssueComment201JSONResponse(postedCommentDTO(posted)), nil
}

func (h *handler) EditWorkspaceIssueComment(
	ctx context.Context,
	request api.EditWorkspaceIssueCommentRequestObject,
) (api.EditWorkspaceIssueCommentResponseObject, error) {
	comment, err := h.issueComments.Edit(
		ctx, request.WorkspaceId, request.IssueId, request.CommentId,
		service.EditCommentInput{Body: request.Body.Body},
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.EditWorkspaceIssueComment200JSONResponse(commentDTO(comment)), nil
}

func (h *handler) RemoveWorkspaceIssueComment(
	ctx context.Context,
	request api.RemoveWorkspaceIssueCommentRequestObject,
) (api.RemoveWorkspaceIssueCommentResponseObject, error) {
	err := h.issueComments.Remove(ctx, request.WorkspaceId, request.IssueId, request.CommentId)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.RemoveWorkspaceIssueComment204Response{}, nil
}

func (h *handler) ReactToWorkspaceIssueComment(
	ctx context.Context,
	request api.ReactToWorkspaceIssueCommentRequestObject,
) (api.ReactToWorkspaceIssueCommentResponseObject, error) {
	comment, err := h.issueComments.React(
		ctx, request.WorkspaceId, request.IssueId, request.CommentId,
		entity.CommentReaction(request.Reaction),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.ReactToWorkspaceIssueComment200JSONResponse(commentDTO(comment)), nil
}

func (h *handler) UnreactToWorkspaceIssueComment(
	ctx context.Context,
	request api.UnreactToWorkspaceIssueCommentRequestObject,
) (api.UnreactToWorkspaceIssueCommentResponseObject, error) {
	comment, err := h.issueComments.Unreact(
		ctx, request.WorkspaceId, request.IssueId, request.CommentId,
		entity.CommentReaction(request.Reaction),
	)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	return api.UnreactToWorkspaceIssueComment200JSONResponse(commentDTO(comment)), nil
}
