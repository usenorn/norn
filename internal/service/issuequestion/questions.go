package issuequestion

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
	"github.com/usenorn/norn/internal/service"
)

type questionsService struct {
	questions   repository.IssueQuestion
	issues      repository.Issue
	delegations repository.IssueDelegation
	activity    repository.Activity
	notify      repository.NotificationEvent
	executions  service.Executions
	events      service.Events
	transactor  repository.Transactor
	authorizer  service.Authorizer
}

func New(
	questions repository.IssueQuestion,
	issues repository.Issue,
	delegations repository.IssueDelegation,
	activity repository.Activity,
	notify repository.NotificationEvent,
	executions service.Executions,
	events service.Events,
	transactor repository.Transactor,
	authorizer service.Authorizer,
) service.IssueQuestions {
	return &questionsService{
		questions:   questions,
		issues:      issues,
		delegations: delegations,
		activity:    activity,
		notify:      notify,
		executions:  executions,
		events:      events,
		transactor:  transactor,
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

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	return s.ask(ctx, asking{
		workspaceID: issue.WorkspaceID,
		issueID:     issue.ID,
		teamID:      issue.TeamID,
	}, decision.ActivityActor(), decision.Actor.Kind, input)
}

// asking is the little of an issue that recording a question needs, so the channel edge does not
// have to read a whole issue back to name the one its execution already points at.
type asking struct {
	workspaceID uuid.UUID
	issueID     uuid.UUID
	teamID      uuid.UUID
}

func (s *questionsService) ask(
	ctx context.Context,
	target asking,
	attribution entity.ActivityAttribution,
	actorKind entity.ActorKind,
	input service.AskQuestionInput,
) (entity.IssueQuestion, error) {
	if input.Kind == "" {
		input.Kind = entity.QuestionClarification
	}

	if err := entity.NewValidationError(
		entity.ValidateQuestionText("question", input.Question),
		entity.ValidateQuestionKind("kind", input.Kind),
		entity.ValidateQuestionOptions("options", input.Options),
		entity.ValidateQuestionReachable("options", input.Options, input.AllowFreeText),
		entity.ValidateQuestionContext("context", input.Context),
		entity.ValidateQuestionWait("wait", input.Wait),
		defaulted(input),
	); err != nil {
		return entity.IssueQuestion{}, err
	}

	var asked entity.IssueQuestion

	err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		var err error

		asked, err = s.questions.Ask(ctx, entity.IssueQuestion{
			WorkspaceID:      target.workspaceID,
			IssueID:          target.issueID,
			ExecutionID:      input.ExecutionID,
			Ref:              strings.TrimSpace(input.Ref),
			Kind:             input.Kind,
			Blocking:         input.Blocking,
			Options:          trimmed(input.Options),
			AllowFreeText:    input.AllowFreeText,
			Context:          input.Context,
			Question:         strings.TrimSpace(input.Question),
			DefaultAnswer:    strings.TrimSpace(input.Default),
			Deadline:         time.Now().UTC().Add(input.Wait),
			AskedByAccountID: attribution.AccountID,
			ActorKind:        actorKind,
		})
		if err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: target.workspaceID,
			Subject:     entity.IssueSubject(target.issueID),
			Actor:       attribution,
			Kind:        entity.ActivityKindQuestionAsked,
			ToValue:     asked.Question,
		}); err != nil {
			return err
		}

		if err := s.notify.Record(ctx, entity.NotificationEvent{
			WorkspaceID: target.workspaceID,
			Subject:     entity.NotifyIssue(target.issueID),
			Kind:        entity.NotificationKindApprovalWaiting,
			Actor:       attribution.AccountID,
			ActorKind:   actorKind,
			Target:      s.awaitedBy(ctx, target),
		}); err != nil {
			return err
		}

		if asked.ExecutionID != "" {
			if err := s.executions.Questioned(ctx, asked); err != nil {
				return err
			}
		}

		s.announce(ctx, target, asked, entity.EventQuestionAsked)

		return nil
	})
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	return asked, nil
}

// defaulted keeps the rule the table carries: a question the run did not stop for has to say what
// the agent will do meanwhile, and one it did stop for has nothing to declare.
func defaulted(input service.AskQuestionInput) entity.FieldError {
	if input.Blocking {
		return entity.FieldError{}
	}

	return entity.ValidateQuestionAnswer("default", input.Default)
}

func trimmed(options []string) []string {
	kept := make([]string, 0, len(options))

	for _, option := range options {
		kept = append(kept, strings.TrimSpace(option))
	}

	return kept
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

func (s *questionsService) ListByExecution(
	ctx context.Context,
	workspaceID uuid.UUID,
	executionID string,
) ([]entity.IssueQuestion, error) {
	if _, err := s.executions.Visible(ctx, workspaceID, executionID); err != nil {
		return nil, err
	}

	return s.questions.ListByExecution(ctx, workspaceID, executionID)
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

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	held, err := s.questions.GetByID(ctx, workspaceID, questionID)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if held.IssueID != issueID {
		return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
	}

	if !held.Acceptable(input.Answer) {
		return entity.IssueQuestion{}, entity.ErrIssueQuestionUnanswerable
	}

	var answered entity.IssueQuestion

	if err := s.transactor.WithTx(ctx, func(ctx context.Context) error {
		var err error

		answered, err = s.questions.Answer(ctx, workspaceID, repository.QuestionAnswer{
			QuestionID: questionID,
			Answer:     strings.TrimSpace(input.Answer),
			AccountID:  decision.Actor.AccountID,
			AnsweredAt: time.Now().UTC(),
		})
		if err != nil {
			return err
		}

		if err := s.activity.Record(ctx, entity.Activity{
			WorkspaceID: workspaceID,
			Subject:     entity.IssueSubject(issue.ID),
			Actor:       decision.ActivityActor(),
			Kind:        entity.ActivityKindQuestionAnswered,
			FromValue:   answered.Question,
			ToValue:     answered.Answer,
		}); err != nil {
			return err
		}

		s.announce(ctx, subject(issue), answered, entity.EventQuestionSettled)

		return nil
	}); err != nil {
		return entity.IssueQuestion{}, err
	}

	if answered.ExecutionID != "" {
		if err := s.executions.Answered(ctx, answered); err != nil {
			return entity.IssueQuestion{}, err
		}
	}

	return answered, nil
}

func (s *questionsService) Dismiss(
	ctx context.Context,
	workspaceID, issueID, questionID uuid.UUID,
) (entity.IssueQuestion, error) {
	decision, err := s.decide(ctx, workspaceID, entity.ActionManage)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	issue, err := s.issues.GetVisible(ctx, workspaceID, issueID, decision.Scope)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	held, err := s.questions.GetByID(ctx, workspaceID, questionID)
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	if held.IssueID != issueID {
		return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
	}

	dismissed, err := s.questions.Settle(ctx, workspaceID, repository.QuestionSettlement{
		QuestionID: questionID,
		State:      entity.QuestionDismissed,
		AccountID:  decision.Actor.AccountID,
		SettledAt:  time.Now().UTC(),
	})
	if err != nil {
		return entity.IssueQuestion{}, err
	}

	s.announce(ctx, subject(issue), dismissed, entity.EventQuestionSettled)

	if dismissed.Blocking && dismissed.ExecutionID != "" {
		if err := s.executions.Unanswerable(ctx, dismissed, dismissedNote); err != nil {
			return entity.IssueQuestion{}, err
		}
	}

	return dismissed, nil
}

func (s *questionsService) awaitedBy(ctx context.Context, target asking) uuid.UUID {
	delegation, err := s.delegations.Open(ctx, target.workspaceID, target.issueID)
	if err != nil {
		return uuid.Nil
	}

	return delegation.DelegatedByAccountID
}

func (s *questionsService) announce(
	ctx context.Context,
	target asking,
	question entity.IssueQuestion,
	kind entity.EventKind,
) {
	payload, err := json.Marshal(question)
	if err != nil {
		return
	}

	postgres.AfterCommit(ctx, func(ctx context.Context) {
		s.events.Publish(ctx, entity.Event{
			WorkspaceID: target.workspaceID,
			Kind:        kind,
			TeamID:      target.teamID,
			SubjectID:   target.issueID,
			IssueID:     target.issueID,
			Payload:     payload,
		})
	})
}

func subject(issue entity.Issue) asking {
	return asking{workspaceID: issue.WorkspaceID, issueID: issue.ID, teamID: issue.TeamID}
}
