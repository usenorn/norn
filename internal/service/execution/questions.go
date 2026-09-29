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
		return s.tell(ctx, execution, entity.ChannelQuestionAnswered, answerOf(question))
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
	listed, err := s.questions.ListByExecution(ctx, execution.WorkspaceID, execution.ID)
	if err != nil {
		return err
	}

	questions := settledIn(listed, question)

	if len(entity.BlockingQuestionsOpen(questions)) > 0 {
		return nil
	}

	resumed, err := s.advance(ctx, execution, move{
		to:     entity.ExecutionQueuedForResume,
		reason: answeredNote(question),
		actor:  settler(question),
	})
	if err != nil {
		return err
	}

	if err := s.tell(ctx, resumed, entity.ChannelExecutionResume, channelv1.Instruction{
		Reason:  channelv1.ResumeAnswer,
		Stage:   resumed.Stage,
		Answers: answersOf(questions),
	}); err != nil {
		return err
	}

	s.record(ctx, entity.AuditExecutionResumed, resumed)

	return nil
}

func settledIn(listed []entity.IssueQuestion, settled entity.IssueQuestion) []entity.IssueQuestion {
	questions := make([]entity.IssueQuestion, 0, len(listed)+1)
	found := false

	for _, question := range listed {
		if question.ID == settled.ID {
			question, found = settled, true
		}

		questions = append(questions, question)
	}

	if !found {
		questions = append(questions, settled)
	}

	return questions
}

func answersOf(questions []entity.IssueQuestion) []channelv1.Answer {
	answers := make([]channelv1.Answer, 0, len(questions))

	for _, question := range questions {
		if !question.Answered() {
			continue
		}

		answers = append(answers, answerOf(question))
	}

	return answers
}

func answerOf(question entity.IssueQuestion) channelv1.Answer {
	answer := channelv1.Answer{
		QuestionID: question.ID.String(),
		Ref:        question.Ref,
		Question:   question.Question,
		Answer:     question.Answer,
		AnsweredBy: question.AnsweredByName,
	}

	if question.AnsweredAt != nil {
		answer.AnsweredAt = *question.AnsweredAt
	}

	return answer
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
