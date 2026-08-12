package issuequestion

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type questionsService struct {
	questions   repository.IssueQuestion
	issues      repository.Issue
	delegations repository.IssueDelegation
	notify      repository.NotificationEvent
	authorizer  service.Authorizer
}

func New(
	questions repository.IssueQuestion,
	issues repository.Issue,
	delegations repository.IssueDelegation,
	notify repository.NotificationEvent,
	authorizer service.Authorizer,
) service.IssueQuestions {
	return &questionsService{
		questions:   questions,
		issues:      issues,
		delegations: delegations,
		notify:      notify,
		authorizer:  authorizer,
	}
}

func (s *questionsService) decide(
	ctx context.Context,
	workspaceID uuid.UUID,
	action entity.Action,
) (entity.Decision, error) {
	return s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceIssue,
		Action:      action,
		WorkspaceID: workspaceID,
		Scoped:      true,
	})
}

func (s *questionsService) Ask(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	input service.AskQuestionInput,
) (entity.IssueQuestion, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateQuestionText("question", input.Question),
		entity.ValidateQuestionAnswer("default", input.Default),
		entity.ValidateQuestionWait("wait", input.Wait),
	); err != nil {
		return entity.IssueQuestion{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	asked, err := s.questions.Ask(ctx, entity.IssueQuestion{
		WorkspaceID:      workspaceID,
		IssueID:          issue.ID,
		Question:         strings.TrimSpace(input.Question),
		DefaultAnswer:    strings.TrimSpace(input.Default),
		Deadline:         time.Now().UTC().Add(input.Wait),
		AskedByAccountID: decision.Actor.AccountID,
		ActorKind:        decision.Actor.Kind,
	})
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if err := s.notify.Record(ctx, entity.NotificationEvent{
		WorkspaceID: workspaceID,
		Subject:     entity.NotifyIssue(issue.ID),
		Kind:        entity.NotificationKindApprovalWaiting,
		Actor:       decision.Actor.AccountID,
		ActorKind:   decision.Actor.Kind,
		Target:      s.awaitedBy(ctx, issue),
	}); err != nil {
		return entity.IssueQuestion{}, err
	}

	return asked, nil
}

func (s *questionsService) List(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) ([]entity.IssueQuestion, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionRead)
	if err != nil {
		return nil, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return nil, err
	}

	return s.questions.ListByIssue(ctx, workspaceID, issueID)
}

func (s *questionsService) Answer(
	ctx context.Context,
	workspaceID, issueID, questionID uuid.UUID,
	input service.AnswerQuestionInput,
) (entity.IssueQuestion, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateQuestionAnswer("answer", input.Answer),
	); err != nil {
		return entity.IssueQuestion{}, err
	}

	if _, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope); err != nil {
		return entity.IssueQuestion{}, err
	}

	held, err := s.questions.GetByID(ctx, workspaceID, questionID)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if held.IssueID != issueID {
		return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
	}

	return s.questions.Answer(ctx, workspaceID, repository.QuestionAnswer{
		QuestionID: questionID,
		Answer:     strings.TrimSpace(input.Answer),
		AccountID:  decision.Actor.AccountID,
		AnsweredAt: time.Now().UTC(),
	})
}

func (s *questionsService) awaitedBy(ctx context.Context, issue entity.Issue) uuid.UUID {
	delegation, err := s.delegations.Open(ctx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return uuid.Nil
	}

	return delegation.DelegatedByAccountID
}
