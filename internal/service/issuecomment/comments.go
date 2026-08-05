package issuecomment

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

const mentionsField = "mentions"

type issueCommentsService struct {
	comments    repository.IssueComment
	attachments repository.Attachment
	issues      repository.Issue
	teams       repository.Team
	activity    repository.Activity
	notify      repository.NotificationEvent
	events      service.Events
	followers   repository.IssueFollower
	authorizer  service.Authorizer
	transactor  repository.Transactor
}

func New(
	comments repository.IssueComment,
	attachments repository.Attachment,
	issues repository.Issue,
	teams repository.Team,
	activity repository.Activity,
	notify repository.NotificationEvent,
	events service.Events,
	followers repository.IssueFollower,
	authorizer service.Authorizer,
	transactor repository.Transactor,
) service.IssueComments {
	return &issueCommentsService{
		comments:    comments,
		attachments: attachments,
		issues:      issues,
		teams:       teams,
		activity:    activity,
		notify:      notify,
		events:      events,
		followers:   followers,
		authorizer:  authorizer,
		transactor:  transactor,
	}
}

func (s *issueCommentsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceComment,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *issueCommentsService) onVisibleIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	action entity.Action,
) (entity.Decision, entity.Issue, error) {
	decision, err := s.decide(ctx, workspaceID, action)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.Decision{}, entity.Issue{}, err
	}

	return decision, issue, nil
}

func (s *issueCommentsService) reachable(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	action entity.Action,
) (entity.Decision, entity.IssueComment, error) {
	decision, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, action)
	if err != nil {
		return entity.Decision{}, entity.IssueComment{}, err
	}

	comment, err := s.comments.GetByID(ctx, workspaceID, commentID)
	if err != nil {
		return entity.Decision{}, entity.IssueComment{}, err
	}

	if comment.IssueID != issueID {
		return entity.Decision{}, entity.IssueComment{}, entity.ErrIssueCommentNotFound
	}

	return decision, comment, nil
}

func (s *issueCommentsService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.ListCommentsInput,
) (service.CommentThread, error) {
	if _, _, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionRead); err != nil {
		return service.CommentThread{}, err
	}

	page := entity.CommentPage{Limit: input.Limit}.Normalized()

	switch {
	case input.Around != uuid.Nil:
		cursor, err := s.comments.CursorBefore(ctx, issueID, input.Around)
		if err != nil {
			return service.CommentThread{}, err
		}

		page.Cursor = cursor
	case input.Cursor != "":
		cursor, err := entity.DecodeCommentCursor(input.Cursor)
		if err != nil {
			return service.CommentThread{}, err
		}

		page.Cursor = &cursor
	}

	comments, err := s.comments.ListThread(ctx, issueID, page.Lookahead())
	if err != nil {
		return service.CommentThread{}, err
	}

	thread := service.CommentThread{Comments: comments}

	if len(comments) > page.Limit {
		thread.Comments = comments[:page.Limit]
		thread.NextCursor = thread.Comments[page.Limit-1].Cursor().Encode()
	}

	return thread, nil
}

func (s *issueCommentsService) Post(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.PostCommentInput,
) (service.CommentPosted, error) {
	decision, issue, err := s.onVisibleIssue(ctx, workspaceID, issueID, entity.ActionManage)
	if err != nil {
		return service.CommentPosted{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateCommentBody("body", input.Body),
		entity.ValidateMentionCount(mentionsField, len(input.Mentions)),
	); err != nil {
		return service.CommentPosted{}, err
	}

	if err := s.replyable(ctx, workspaceID, issueID, input.ParentCommentID); err != nil {
		return service.CommentPosted{}, err
	}

	mentions, err := s.resolve(ctx, workspaceID, issue.TeamID, decision, input.Mentions)
	if err != nil {
		return service.CommentPosted{}, err
	}

	var posted entity.IssueComment

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		posted, err = s.comments.Create(ctx, entity.IssueComment{
			WorkspaceID:     workspaceID,
			IssueID:         issueID,
			ParentCommentID: input.ParentCommentID,
			AuthorAccountID: decision.Actor.AccountID,
			Body:            input.Body,
		})
		if err != nil {
			return err
		}

		if err := s.comments.RecordMentions(ctx, posted.ID, mentions); err != nil {
			return err
		}

		if err := s.attachments.ClaimForComment(
			ctx, workspaceID, issueID, posted.ID, input.AttachmentIDs,
		); err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issueID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindCommented,
		}); err != nil {
			return err
		}

		for _, follower := range append(mentionedAccounts(mentions), decision.Actor.AccountID) {
			if err := s.followers.Follow(ctx, entity.IssueFollower{
				IssueID:     issueID,
				WorkspaceID: workspaceID,
				AccountID:   follower,
			}); err != nil {
				return err
			}
		}

		attribution := decision.ActivityActor()

		if err := s.notify.Record(ctx, entity.NotificationEvent{
			WorkspaceID: workspaceID,
			Subject:     entity.NotifyIssue(issueID),
			Kind:        entity.NotificationKindCommented,
			Actor:       attribution.AccountID,
			ActorKind:   attribution.Kind,
			CommentID:   posted.ID,
		}); err != nil {
			return err
		}

		s.broadcast(ctx, entity.EventCommentPosted, issue, posted)

		return nil
	}); err != nil {
		return service.CommentPosted{}, err
	}

	posted.Mentions = mentions

	return service.CommentPosted{Comment: posted, Unreachable: unreachable(mentions)}, nil
}

func (s *issueCommentsService) replyable(
	ctx context.Context,
	workspaceID, issueID, parentCommentID uuid.UUID,
) error {
	if parentCommentID == uuid.Nil {
		return nil
	}

	parent, err := s.comments.GetByID(ctx, workspaceID, parentCommentID)
	if err != nil {
		return err
	}

	if parent.IssueID != issueID {
		return entity.ErrIssueCommentNotFound
	}

	if !parent.Root() {
		return entity.ErrIssueCommentNotReplyable
	}

	return nil
}

func mentionedAccounts(mentions []entity.CommentMention) []uuid.UUID {
	accounts := make([]uuid.UUID, 0, len(mentions))

	for _, mention := range mentions {
		if mention.Visible && mention.Kind == entity.MentionKindAccount {
			accounts = append(accounts, mention.AccountID)
		}
	}

	return accounts
}

func unreachable(mentions []entity.CommentMention) []entity.CommentMention {
	missed := make([]entity.CommentMention, 0)

	for _, mention := range mentions {
		if !mention.Visible {
			missed = append(missed, mention)
		}
	}

	return missed
}

func (s *issueCommentsService) resolve(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
	decision entity.Decision,
	requested []service.CommentMentionInput,
) ([]entity.CommentMention, error) {
	mentions := make([]entity.CommentMention, 0, len(requested))

	if len(requested) == 0 {
		return mentions, nil
	}

	accountIDs := make([]uuid.UUID, 0, len(requested))

	for _, mention := range requested {
		if mention.Kind == entity.MentionKindAccount {
			accountIDs = append(accountIDs, mention.AccountID)
		}
	}

	audience, err := s.comments.Audience(ctx, workspaceID, teamID, accountIDs)
	if err != nil {
		return nil, err
	}

	known := make(map[uuid.UUID]repository.CommentAudience, len(audience))
	for _, member := range audience {
		known[member.AccountID] = member
	}

	for _, mention := range requested {
		if mention.Kind == entity.MentionKindTeam {
			resolved, err := s.mentionedTeam(ctx, workspaceID, decision, mention.TeamID)
			if err != nil {
				return nil, err
			}

			mentions = append(mentions, resolved)

			continue
		}

		member, ok := known[mention.AccountID]
		if !ok {
			return nil, unmentionable()
		}

		mentions = append(mentions, entity.CommentMention{
			Kind:      entity.MentionKindAccount,
			AccountID: member.AccountID,
			Name:      member.Name,
			Visible:   member.Visible,
		})
	}

	return mentions, nil
}

func (s *issueCommentsService) mentionedTeam(
	ctx context.Context,
	workspaceID uuid.UUID,
	decision entity.Decision,
	teamID uuid.UUID,
) (entity.CommentMention, error) {
	team, err := s.teams.GetByID(ctx, teamID)
	if err != nil {
		return entity.CommentMention{}, err
	}

	if team.WorkspaceID != workspaceID || !decision.Scope.Covers(team.ID) {
		return entity.CommentMention{}, unmentionable()
	}

	return entity.CommentMention{
		Kind:    entity.MentionKindTeam,
		TeamID:  team.ID,
		Name:    team.Name,
		Visible: true,
	}, nil
}

func unmentionable() error {
	return entity.NewValidationError(entity.FieldError{
		Field: mentionsField,
		Code:  entity.ValidationCodeUnsupportedValue,
	})
}

func (s *issueCommentsService) Edit(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	input service.EditCommentInput,
) (entity.IssueComment, error) {
	decision, comment, err := s.reachable(ctx, workspaceID, issueID, commentID, entity.ActionManage)
	if err != nil {
		return entity.IssueComment{}, err
	}

	if comment.Deleted() {
		return entity.IssueComment{}, entity.ErrIssueCommentDeleted
	}

	if !comment.EditableBy(decision.Actor.AccountID) {
		return entity.IssueComment{}, entity.ErrIssueCommentNotAuthor
	}

	if err := entity.NewValidationError(
		entity.ValidateCommentBody("body", input.Body),
	); err != nil {
		return entity.IssueComment{}, err
	}

	if err := s.comments.Edit(ctx, commentID, input.Body, time.Now().UTC()); err != nil {
		return entity.IssueComment{}, err
	}

	return s.comments.GetByID(ctx, workspaceID, commentID)
}

func (s *issueCommentsService) Remove(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
) error {
	decision, comment, err := s.reachable(ctx, workspaceID, issueID, commentID, entity.ActionManage)
	if err != nil {
		return err
	}

	if comment.Deleted() {
		return entity.ErrIssueCommentDeleted
	}

	if !comment.DeletableBy(decision.Actor.AccountID, decision.Role) {
		return entity.ErrIssueCommentNotDeletable
	}

	return s.transactor.WithTx(ctx, func(ctx context.Context) error {
		if err := s.comments.Tombstone(ctx, commentID, time.Now().UTC()); err != nil {
			return err
		}

		return s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(comment.IssueID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindCommentDeleted,
		})
	})
}

func (s *issueCommentsService) React(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	reaction entity.CommentReaction,
) (entity.IssueComment, error) {
	return s.reacted(ctx, workspaceID, issueID, commentID, reaction, s.comments.React)
}

func (s *issueCommentsService) Unreact(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	reaction entity.CommentReaction,
) (entity.IssueComment, error) {
	return s.reacted(ctx, workspaceID, issueID, commentID, reaction, s.comments.Unreact)
}

func (s *issueCommentsService) reacted(
	ctx context.Context,
	workspaceID, issueID, commentID uuid.UUID,
	reaction entity.CommentReaction,
	apply func(context.Context, uuid.UUID, uuid.UUID, entity.CommentReaction) error,
) (entity.IssueComment, error) {
	decision, comment, err := s.reachable(ctx, workspaceID, issueID, commentID, entity.ActionManage)
	if err != nil {
		return entity.IssueComment{}, err
	}

	if comment.Deleted() {
		return entity.IssueComment{}, entity.ErrIssueCommentDeleted
	}

	if !reaction.Valid() {
		return entity.IssueComment{}, entity.NewValidationError(entity.FieldError{
			Field: "reaction",
			Code:  entity.ValidationCodeUnsupportedValue,
		})
	}

	if err := apply(ctx, commentID, decision.Actor.AccountID, reaction); err != nil {
		return entity.IssueComment{}, err
	}

	return s.comments.GetByID(ctx, workspaceID, commentID)
}
