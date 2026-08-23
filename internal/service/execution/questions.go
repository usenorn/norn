package execution

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func (s *executionsService) Questioned(ctx context.Context, question entity.IssueQuestion) error {
	execution, err := s.executions.GetByID(ctx, question.ExecutionID)
	if err != nil {
		return err
	}

	return s.remember(ctx, execution, entity.ExecutionEvent{
		ExecutionID: execution.ID,
		Kind:        entity.ExecutionEventQuestion,
		Actor:       runnerActor(entity.Runner{AgentID: execution.AgentID, ID: execution.RunnerID}),
		Reason:      question.Question,
		SourceID:    question.Ref,
		OccurredAt:  question.CreatedAt,
	})
}

func (s *executionsService) Answered(ctx context.Context, question entity.IssueQuestion) error {
	execution, err := s.executions.GetByID(ctx, question.ExecutionID)
	if err != nil {
		return err
	}

	if execution.RunnerID == uuid.Nil {
		return nil
	}

	answer := channelv1.Answer{
		QuestionID: question.ID.String(),
		Ref:        question.Ref,
		Answer:     question.Answer,
		AnsweredBy: question.AnsweredByName,
	}

	if question.AnsweredAt != nil {
		answer.AnsweredAt = *question.AnsweredAt
	}

	if err := s.remember(ctx, execution, entity.ExecutionEvent{
		ExecutionID: execution.ID,
		Kind:        entity.ExecutionEventQuestion,
		Actor:       settler(question),
		Reason:      answeredNote(question),
		SourceID:    question.ID.String(),
		OccurredAt:  settledAt(question),
	}); err != nil {
		return err
	}

	switch execution.State {
	case entity.ExecutionRunning:
		return s.tell(ctx, execution, entity.ChannelQuestionAnswered, answer)
	case entity.ExecutionWaitingForInput:
		return s.wake(ctx, execution, question)
	default:
		return nil
	}
}

func settledAt(question entity.IssueQuestion) time.Time {
	if question.SettledAt != nil {
		return *question.SettledAt
	}

	return time.Now().UTC()
}

func (s *executionsService) wake(
	ctx context.Context,
	execution entity.Execution,
	question entity.IssueQuestion,
) error {
	resumed, err := s.advance(ctx, execution, move{
		to:     entity.ExecutionQueuedForResume,
		reason: answeredNote(question),
		actor:  settler(question),
	})
	if err != nil {
		return err
	}

	if err := s.tell(ctx, resumed, entity.ChannelExecutionResume, channelv1.Instruction{
		Reason:      channelv1.ResumeAnswer,
		Instruction: question.Answer,
		QuestionID:  question.ID.String(),
		QuestionRef: question.Ref,
	}); err != nil {
		return err
	}

	s.record(ctx, entity.AuditExecutionResumed, resumed)

	return nil
}

func (s *executionsService) Unanswerable(
	ctx context.Context,
	question entity.IssueQuestion,
	reason string,
) error {
	execution, err := s.executions.GetByID(ctx, question.ExecutionID)
	if err != nil {
		return err
	}

	if execution.State != entity.ExecutionWaitingForInput {
		return nil
	}

	stranded, err := s.advance(ctx, execution, move{
		to:     entity.ExecutionFailed,
		reason: reason,
		actor:  settler(question),
	})
	if err != nil {
		return err
	}

	s.record(ctx, entity.AuditExecutionStranded, stranded)

	return nil
}

// settler names whoever decided this question rather than the run it belonged to, so a run stopped
// because somebody declined to answer says who declined rather than blaming the machine.
func settler(question entity.IssueQuestion) entity.ExecutionActor {
	if question.SettledBy == uuid.Nil {
		return entity.SystemExecutionActor()
	}

	return entity.ExecutionActor{Kind: entity.ActorKindUser, AccountID: question.SettledBy}
}

func answeredNote(question entity.IssueQuestion) string {
	who := question.SettledByName
	if who == "" {
		who = "somebody"
	}

	return who + " answered: " + question.Answer
}
