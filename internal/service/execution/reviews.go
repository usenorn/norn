package execution

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) Review(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (service.ExecutionReviewState, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionRead)
	if err != nil {
		return service.ExecutionReviewState{}, err
	}

	heads, err := s.heads(ctx, execution)
	if err != nil {
		return service.ExecutionReviewState{}, err
	}

	comments, err := s.reviews.ListComments(ctx, execution.ID)
	if err != nil {
		return service.ExecutionReviewState{}, err
	}

	reviews, err := s.reviews.ListReviews(ctx, execution.ID)
	if err != nil {
		return service.ExecutionReviewState{}, err
	}

	visible := make([]entity.ExecutionReviewComment, 0, len(comments))

	for _, comment := range comments {
		if comment.VisibleTo(decision.Actor.AccountID) {
			visible = append(visible, comment)
		}
	}

	return service.ExecutionReviewState{Heads: heads, Comments: visible, Reviews: reviews}, nil
}

func (s *executionsService) heads(
	ctx context.Context,
	execution entity.Execution,
) (entity.ReviewHeads, error) {
	changeset, err := s.changesets.Get(ctx, execution.ID)
	if err != nil {
		return nil, err
	}

	return entity.HeadsOf(changeset.Changes), nil
}

func (s *executionsService) reviewing(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) (entity.Decision, entity.Execution, error) {
	decision, execution, err := s.visible(ctx, workspaceID, executionID, entity.ActionManage)
	if err != nil {
		return entity.Decision{}, entity.Execution{}, err
	}

	if execution.State != entity.ExecutionAwaitingReview {
		return entity.Decision{}, entity.Execution{}, entity.ErrReviewClosed
	}

	if decision.Actor.AccountID == uuid.Nil {
		return entity.Decision{}, entity.Execution{}, entity.ErrReviewCommentNotYours
	}

	return decision, execution, nil
}

func (s *executionsService) CommentOnReview(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	draft service.ReviewCommentDraft,
) (entity.ExecutionReviewComment, error) {
	decision, execution, err := s.reviewing(ctx, workspaceID, executionID)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateReviewCommentBody("body", draft.Body),
	); err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	anchor, err := s.anchor(ctx, execution, draft)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	count, err := s.reviews.CountComments(ctx, execution.ID)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	if count >= entity.ReviewCommentsPerRunMax {
		return entity.ExecutionReviewComment{}, entity.ErrReviewCommentsFull
	}

	now := time.Now().UTC()

	var added entity.ExecutionReviewComment

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		comment := entity.ExecutionReviewComment{
			ExecutionID:     execution.ID,
			WorkspaceID:     execution.WorkspaceID,
			ParentID:        draft.ParentID,
			Anchor:          anchor,
			Body:            strings.TrimSpace(draft.Body),
			AuthorAccountID: decision.Actor.AccountID,
			CreatedAt:       now,
		}

		if draft.Publish {
			review, err := s.reviews.CreateReview(ctx, entity.ExecutionReview{
				ExecutionID:     execution.ID,
				WorkspaceID:     execution.WorkspaceID,
				Verdict:         entity.VerdictComment,
				Heads:           entity.ReviewHeads{anchor.Repository: anchor.HeadSHA},
				AuthorAccountID: decision.Actor.AccountID,
				SubmittedAt:     now,
			})
			if err != nil {
				return err
			}

			comment.ReviewID = review.ID
		}

		added, err = s.reviews.AddComment(ctx, comment)
		if err != nil {
			return err
		}

		if !added.Pending() {
			postgres.AfterCommit(ctx, func(ctx context.Context) {
				s.publish(ctx, entity.EventExecutionReview, execution)
			})
		}

		return nil
	})
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	return added, nil
}

func (s *executionsService) anchor(
	ctx context.Context,
	execution entity.Execution,
	draft service.ReviewCommentDraft,
) (entity.ReviewAnchor, error) {
	if draft.ParentID != uuid.Nil {
		parent, err := s.reviews.GetComment(ctx, execution.ID, draft.ParentID)
		if err != nil {
			return entity.ReviewAnchor{}, err
		}

		if parent.Reply() {
			return entity.ReviewAnchor{}, entity.ErrReviewCommentReply
		}

		return parent.Anchor, nil
	}

	anchor := draft.Anchor
	anchor.Hunk = entity.TrimReviewHunk(anchor.Hunk)

	if err := entity.ValidateReviewAnchor(anchor); err != nil {
		return entity.ReviewAnchor{}, err
	}

	heads, err := s.heads(ctx, execution)
	if err != nil {
		return entity.ReviewAnchor{}, err
	}

	head, ok := heads[anchor.Repository]
	if !ok {
		return entity.ReviewAnchor{}, entity.ErrReviewCommentAnchor
	}

	anchor.HeadSHA = head

	return anchor, nil
}

func (s *executionsService) own(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	commentID uuid.UUID,
) (entity.Execution, entity.ExecutionReviewComment, error) {
	decision, execution, err := s.reviewing(ctx, workspaceID, executionID)
	if err != nil {
		return entity.Execution{}, entity.ExecutionReviewComment{}, err
	}

	comment, err := s.reviews.GetComment(ctx, execution.ID, commentID)
	if err != nil {
		return entity.Execution{}, entity.ExecutionReviewComment{}, err
	}

	if !comment.VisibleTo(decision.Actor.AccountID) {
		return entity.Execution{}, entity.ExecutionReviewComment{}, entity.ErrReviewCommentNotFound
	}

	if comment.AuthorAccountID != decision.Actor.AccountID {
		return entity.Execution{}, entity.ExecutionReviewComment{}, entity.ErrReviewCommentNotYours
	}

	return execution, comment, nil
}

func (s *executionsService) EditReviewComment(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	commentID uuid.UUID,
	body string,
) (entity.ExecutionReviewComment, error) {
	execution, comment, err := s.own(ctx, workspaceID, executionID, commentID)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateReviewCommentBody("body", body),
	); err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	edited, err := s.reviews.EditComment(ctx, repository.ReviewCommentEdit{
		ExecutionID: execution.ID,
		CommentID:   comment.ID,
		Body:        strings.TrimSpace(body),
		At:          time.Now().UTC(),
	})
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	if !edited.Pending() {
		s.publish(ctx, entity.EventExecutionReview, execution)
	}

	return edited, nil
}

func (s *executionsService) DeleteReviewComment(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	commentID uuid.UUID,
) error {
	execution, comment, err := s.own(ctx, workspaceID, executionID, commentID)
	if err != nil {
		return err
	}

	if err := s.reviews.DeleteComment(ctx, execution.ID, comment.ID); err != nil {
		return err
	}

	if !comment.Pending() {
		s.publish(ctx, entity.EventExecutionReview, execution)
	}

	return nil
}

func (s *executionsService) ResolveReviewComment(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	commentID uuid.UUID,
	resolved bool,
) (entity.ExecutionReviewComment, error) {
	decision, execution, err := s.reviewing(ctx, workspaceID, executionID)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	comment, err := s.reviews.GetComment(ctx, execution.ID, commentID)
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	if comment.Pending() || comment.Reply() {
		return entity.ExecutionReviewComment{}, entity.ErrReviewCommentNotFound
	}

	settled, err := s.reviews.ResolveComment(ctx, repository.ReviewResolution{
		ExecutionID: execution.ID,
		CommentID:   comment.ID,
		AccountID:   decision.Actor.AccountID,
		Resolved:    resolved,
		At:          time.Now().UTC(),
	})
	if err != nil {
		return entity.ExecutionReviewComment{}, err
	}

	s.publish(ctx, entity.EventExecutionReview, execution)

	return settled, nil
}

func (s *executionsService) SubmitReview(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
	submission service.ReviewSubmission,
) (entity.ExecutionReview, error) {
	decision, execution, err := s.reviewing(ctx, workspaceID, executionID)
	if err != nil {
		return entity.ExecutionReview{}, err
	}

	if !submission.Verdict.Valid() {
		return entity.ExecutionReview{}, entity.NewValidationError(
			entity.FieldError{Field: "verdict", Code: entity.ValidationCodeUnsupportedValue},
		)
	}

	if err := entity.NewValidationError(
		entity.ValidateReviewSummary("summary", submission.Summary),
	); err != nil {
		return entity.ExecutionReview{}, err
	}

	if acting := decision.Actor.AgentID; acting != nil && *acting == execution.AgentID {
		return entity.ExecutionReview{}, entity.ErrExecutionSelfApproval
	}

	heads, err := s.heads(ctx, execution)
	if err != nil {
		return entity.ExecutionReview{}, err
	}

	if !heads.Matches(submission.Heads) {
		return entity.ExecutionReview{}, entity.ErrReviewStale
	}

	comments, err := s.reviews.ListComments(ctx, execution.ID)
	if err != nil {
		return entity.ExecutionReview{}, err
	}

	pending := make([]entity.ExecutionReviewComment, 0)

	for _, comment := range comments {
		if comment.Pending() && comment.AuthorAccountID == decision.Actor.AccountID {
			pending = append(pending, comment)
		}
	}

	if len(pending) > entity.ReviewCommentsPerReview {
		return entity.ExecutionReview{}, entity.ErrReviewCommentsFull
	}

	summary := strings.TrimSpace(submission.Summary)

	if submission.Verdict != entity.VerdictApprove && summary == "" && len(pending) == 0 {
		return entity.ExecutionReview{}, entity.ErrReviewEmpty
	}

	if submission.Verdict != entity.VerdictComment {
		if err := s.settled(ctx, execution); err != nil {
			return entity.ExecutionReview{}, err
		}
	}

	var (
		review entity.ExecutionReview
		moved  entity.Execution
	)

	err = s.transactor.WithTx(ctx, func(ctx context.Context) error {
		review, err = s.reviews.CreateReview(ctx, entity.ExecutionReview{
			ExecutionID:     execution.ID,
			WorkspaceID:     execution.WorkspaceID,
			Verdict:         submission.Verdict,
			Summary:         summary,
			Heads:           heads,
			AuthorAccountID: decision.Actor.AccountID,
			SubmittedAt:     time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		if err := s.reviews.AttachPending(
			ctx, execution.ID, decision.Actor.AccountID, review.ID,
		); err != nil {
			return err
		}

		moved, err = s.conclude(ctx, execution, decision, submission.Verdict, summary)
		if err != nil {
			return err
		}

		postgres.AfterCommit(ctx, func(ctx context.Context) {
			s.publish(ctx, entity.EventExecutionReview, execution)
		})

		return nil
	})
	if err != nil {
		return entity.ExecutionReview{}, err
	}

	if err := s.handBack(ctx, moved, submission.Verdict, summary, pending); err != nil {
		return entity.ExecutionReview{}, err
	}

	return review, nil
}

func (s *executionsService) conclude(
	ctx context.Context,
	execution entity.Execution,
	decision entity.Decision,
	verdict entity.ExecutionReviewVerdict,
	summary string,
) (entity.Execution, error) {
	switch verdict {
	case entity.VerdictApprove:
		return s.advance(ctx, execution, move{
			to:     entity.ExecutionApproved,
			reason: summary,
			actor:  entity.ExecutionActorOf(decision.Actor),
		})
	case entity.VerdictRequestChanges:
		return s.advance(ctx, execution, move{
			to:     entity.ExecutionQueuedForResume,
			reason: firstOf(summary, "changes requested"),
			actor:  entity.ExecutionActorOf(decision.Actor),
		})
	default:
		return execution, nil
	}
}

func (s *executionsService) handBack(
	ctx context.Context,
	execution entity.Execution,
	verdict entity.ExecutionReviewVerdict,
	summary string,
	comments []entity.ExecutionReviewComment,
) error {
	if verdict == entity.VerdictComment {
		return nil
	}

	if execution.RunnerID != uuid.Nil {
		instruction := channelv1.Instruction{Reason: channelv1.ResumeApproved, Stage: execution.Stage}

		if verdict == entity.VerdictRequestChanges {
			instruction.Reason = channelv1.ResumeFeedback
			instruction.Instruction = entity.ComposeReviewFeedback(summary, comments)
		}

		if err := s.tell(ctx, execution, entity.ChannelExecutionResume, instruction); err != nil {
			return err
		}
	}

	if verdict == entity.VerdictApprove {
		s.record(ctx, entity.AuditExecutionApproved, execution)
	} else {
		s.record(ctx, entity.AuditExecutionResumed, execution)
	}

	return nil
}
