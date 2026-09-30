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
	revision := 0
	if request.Params.Revision != nil {
		revision = *request.Params.Revision
	}

	state, err := h.executions.Review(ctx, request.WorkspaceId, request.ExecutionId, revision)
	if err != nil {
		if problem, ok := problemFor(err); ok {
			return problem, nil
		}

		return nil, err
	}

	viewer, _ := h.currentAccountID(ctx)
	heads := state.Latest.Heads()

	comments := make([]api.ReviewComment, 0, len(state.Comments))
	for _, comment := range state.Comments {
		comments = append(comments, reviewCommentDTO(comment, heads, viewer))
	}

	reviews := make([]api.ExecutionReview, 0, len(state.Reviews))
	for _, review := range state.Reviews {
		reviews = append(reviews, executionReviewDTO(review))
	}

	revisions := make([]api.ReviewRevision, 0, len(state.Revisions))
	for _, held := range state.Revisions {
		revisions = append(revisions, api.ReviewRevision{
			Revision:   held.Revision,
			Additions:  held.Additions,
			Deletions:  held.Deletions,
			ReportedAt: held.ReportedAt,
		})
	}

	return api.GetWorkspaceExecutionReview200JSONResponse(api.ExecutionReviewState{
		Revision:       state.Snapshot.Revision,
		LatestRevision: state.Latest.Revision,
		Revisions:      revisions,
		Summary:        state.Snapshot.Summary,
		Repositories:   reviewRepositoryDTOs(state.Snapshot.Repositories),
		Previews:       reviewPreviewDTOs(state.Snapshot.Previews, state.Sessions, h.previewCfg.Scheme),
		Heads:          reviewHeadDTOs(heads),
		Comments:       comments,
		Reviews:        reviews,
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

	state, err := h.executions.Review(ctx, workspaceID, executionID, 0)
	if err != nil {
		return api.ReviewComment{}, err
	}

	return reviewCommentDTO(comment, state.Latest.Heads(), viewer), nil
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
		Revision:       comment.Revision,
		Body:           comment.Body,
		AuthorName:     nilIfEmpty(comment.AuthorName),
		AuthorKind:     commentAuthorKindDTO(comment.AuthorKind),
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
		Revision:    review.Revision,
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

func commentAuthorKindDTO(kind entity.AccountKind) api.CommentAuthorKind {
	if kind == "" {
		return api.CommentAuthorKind(entity.AccountKindPerson)
	}

	return api.CommentAuthorKind(kind)
}

func reviewRepositoryDTOs(repositories []entity.SnapshotRepository) []api.ReviewRepository {
	dtos := make([]api.ReviewRepository, 0, len(repositories))

	for _, held := range repositories {
		commits := make([]api.ReviewCommit, 0, len(held.Commits))
		for _, commit := range held.Commits {
			commits = append(commits, api.ReviewCommit{Sha: commit.SHA, Subject: commit.Subject})
		}

		dtos = append(dtos, api.ReviewRepository{
			Repository:     held.Repository,
			Branch:         held.Branch,
			BaseSha:        held.BaseSHA,
			HeadSha:        held.HeadSHA,
			Commits:        commits,
			Additions:      held.Additions,
			Deletions:      held.Deletions,
			FilesChanged:   held.FilesChanged,
			DiffArtifactId: nilIfNilID(held.DiffArtifactID),
		})
	}

	return dtos
}

func reviewPreviewDTOs(
	previews []entity.SnapshotPreview,
	sessions []entity.PreviewSession,
	scheme string,
) []api.ReviewPreview {
	dtos := make([]api.ReviewPreview, 0, len(previews))

	for _, preview := range previews {
		dto := api.ReviewPreview{
			Name:    preview.Name,
			Service: preview.Service,
			Path:    nilIfEmpty(preview.Path),
			State:   api.ReviewPreviewState(preview.State),
			Reason:  nilIfEmpty(preview.Reason),
		}

		if preview.State == entity.SnapshotPreviewReady {
			dto.Url = nilIfEmpty(openSessionURL(sessions, preview.Port, scheme))
		}

		dtos = append(dtos, dto)
	}

	return dtos
}

func openSessionURL(sessions []entity.PreviewSession, port int, scheme string) string {
	for _, session := range sessions {
		if session.Port == port && session.Open() {
			return session.URL(scheme)
		}
	}

	return ""
}
