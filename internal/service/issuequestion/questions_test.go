package issuequestion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/identity"
	"github.com/usenorn/norn/internal/service"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func channelQuestion(ref string, blocking bool) channelv1.Question {
	question := channelv1.Question{
		Ref:           ref,
		Kind:          channelv1.QuestionDecision,
		Blocking:      blocking,
		Message:       "Keep the old API endpoint during the migration?",
		Options:       []string{"Keep for 30 days", "Remove now"},
		AllowFreeText: true,
		Asked:         time.Now().UTC(),
	}

	if !blocking {
		question.Default = "Keep for 30 days"
	}

	return question
}

func (h *harness) asRunner(ctx context.Context) context.Context {
	return identity.WithActor(ctx, entity.Actor{
		Kind:      entity.ActorKindAgent,
		AccountID: h.caller,
		AgentID:   &h.execution.AgentID,
		RunnerID:  &h.runner.ID,
	})
}

func TestAQuestionARunAsksLandsOnItsIssueForAPersonToAnswer(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	asked := h.only(t)

	switch {
	case asked.IssueID != h.issue.ID:
		t.Fatalf("the question landed on issue %s, want %s", asked.IssueID, h.issue.ID)
	case asked.ExecutionID != h.execution.ID:
		t.Fatalf("the question points at run %q, want %q", asked.ExecutionID, h.execution.ID)
	case !asked.Blocking:
		t.Fatal("a run that stopped for its question was recorded as one that carried on")
	case len(asked.Options) != 2:
		t.Fatalf("the question kept %d of the answers the agent offered, want 2", len(asked.Options))
	case asked.Kind != entity.QuestionDecision:
		t.Fatalf("the question was filed as %q, want %q", asked.Kind, entity.QuestionDecision)
	}

	if _, ok := h.noted(entity.ActivityKindQuestionAsked); !ok {
		t.Fatal("nothing on the issue timeline says an agent asked anything")
	}
}

func TestARunThatStoppedGetsTheLongDeadlineRatherThanADayToBeNoticed(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	waited := time.Until(h.only(t).Deadline)

	if waited < entity.QuestionParkedWait-time.Minute {
		t.Fatalf(
			"a parked run gets %s to be answered, want about %s; a shorter deadline stops a run "+
				"before anybody has had a chance to look",
			waited, entity.QuestionParkedWait,
		)
	}
}

func TestTheSameQuestionReplayedAfterAReconnectIsNotAskedTwice(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	first := h.asking("01REF", true)

	if err := h.service.Asked(ctx, h.runner, first); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	if err := h.service.Asked(ctx, h.runner, first); err != nil {
		t.Fatalf("a replayed question came back as a failure: %v", err)
	}

	if len(h.stored) != 1 {
		t.Fatalf(
			"a reconnect put %d copies of one question in front of a person, want 1",
			len(h.stored),
		)
	}
}

func TestAQuestionAgainstARunTheMachineDoesNotHoldIsAnsweredNotFound(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	stranger := h.runner
	stranger.ID = h.issue.ID

	err := h.service.Asked(ctx, stranger, h.asking("01REF", true))

	if !errors.Is(err, entity.ErrExecutionNotFound) {
		t.Fatalf(
			"a machine asking against a run it was never given was answered %v; anything but "+
				"not-found lets it probe for other people's runs",
			err,
		)
	}
}

func TestAnAnswerReachesTheRunThatAskedExactlyOnce(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	asked := h.only(t)

	answered, err := h.service.Answer(
		context.Background(), h.workspaceID, h.issue.ID, asked.ID,
		service.AnswerQuestionInput{Answer: "Remove now"},
	)
	if err != nil {
		t.Fatalf("answer the question: %v", err)
	}

	if answered.Answer != "Remove now" || answered.State != entity.QuestionAnswered {
		t.Fatalf("the question now reads %+v, want it answered with what was typed", answered)
	}

	if len(h.answered) != 1 {
		t.Fatalf("the run was told about the answer %d times, want once", len(h.answered))
	}

	entry, noted := h.noted(entity.ActivityKindQuestionAnswered)
	if !noted {
		t.Fatal("the answer is nowhere on the issue timeline, so nobody can see who decided")
	}

	if entry.Actor.AccountID != h.caller || entry.ToValue != "Remove now" {
		t.Fatalf("the timeline recorded %+v, want the answer against whoever gave it", entry)
	}
}

func TestAnAnswerNobodyOfferedIsRefusedWhenTheAgentSaidToPickOne(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	closed := channelQuestion("01REF", true)
	closed.AllowFreeText = false

	if err := h.service.Asked(ctx, h.runner, h.message("01REF", closed)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	_, err := h.service.Answer(
		context.Background(), h.workspaceID, h.issue.ID, h.only(t).ID,
		service.AnswerQuestionInput{Answer: "Do something else entirely"},
	)

	if !errors.Is(err, entity.ErrIssueQuestionUnanswerable) {
		t.Fatalf(
			"free text was accepted against a question that offered a closed set, answering %v; "+
				"the agent would be handed something it has no branch for",
			err,
		)
	}
}

func TestAnsweringAQuestionTwiceIsRefusedRatherThanReplacingTheFirstAnswer(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	asked := h.only(t)
	input := service.AnswerQuestionInput{Answer: "Remove now"}

	if _, err := h.service.Answer(
		context.Background(), h.workspaceID, h.issue.ID, asked.ID, input,
	); err != nil {
		t.Fatalf("answer the question: %v", err)
	}

	_, err := h.service.Answer(
		context.Background(), h.workspaceID, h.issue.ID, asked.ID,
		service.AnswerQuestionInput{Answer: "Keep for 30 days"},
	)

	if !errors.Is(err, entity.ErrIssueQuestionAnswered) {
		t.Fatalf(
			"a second answer was taken, answering %v; the agent would already be working on the "+
				"first one",
			err,
		)
	}
}

func TestDismissingAQuestionARunStoppedForStopsThatRun(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	dismissed, err := h.service.Dismiss(
		context.Background(), h.workspaceID, h.issue.ID, h.only(t).ID,
	)
	if err != nil {
		t.Fatalf("dismiss the question: %v", err)
	}

	if dismissed.State != entity.QuestionDismissed {
		t.Fatalf("the question reads %q, want %q", dismissed.State, entity.QuestionDismissed)
	}

	if dismissed.SettledBy != h.caller {
		t.Fatalf(
			"the question was settled by %s, want %s; a run stopped because somebody declined to "+
				"answer has to say who declined rather than blaming the machine",
			dismissed.SettledBy, h.caller,
		)
	}

	if len(h.stranded) != 1 {
		t.Fatalf(
			"the run was left waiting on a question somebody declined to answer; %d runs were "+
				"stopped, want 1",
			len(h.stranded),
		)
	}
}

func TestDismissingAQuestionARunCarriedOnPastLeavesThatRunAlone(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", false)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	if _, err := h.service.Dismiss(
		context.Background(), h.workspaceID, h.issue.ID, h.only(t).ID,
	); err != nil {
		t.Fatalf("dismiss the question: %v", err)
	}

	if len(h.stranded) != 0 {
		t.Fatal("a run that never stopped was killed because somebody tidied away its question")
	}
}

func TestAQuestionPastItsDeadlineIsMarkedExpiredAndTheRunItHeldIsStopped(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	h.stored[0].Deadline = time.Now().UTC().Add(-time.Hour)

	if err := h.service.SweepExpired(context.Background()); err != nil {
		t.Fatalf("sweep the questions nobody answered: %v", err)
	}

	if h.only(t).State != entity.QuestionExpired {
		t.Fatalf(
			"the question still reads %q past its deadline, so a person is shown a run that is "+
				"still waiting when it is not",
			h.only(t).State,
		)
	}

	if len(h.stranded) != 1 {
		t.Fatalf(
			"%d runs were stopped when their question ran out, want 1; a run left parked holds "+
				"its lease forever with nothing explaining why",
			len(h.stranded),
		)
	}
}

func TestASweptQuestionIsNotSweptAgainOnTheNextRound(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	if err := h.service.Asked(ctx, h.runner, h.asking("01REF", true)); err != nil {
		t.Fatalf("record what a run asked: %v", err)
	}

	h.stored[0].Deadline = time.Now().UTC().Add(-time.Hour)

	for range 2 {
		if err := h.service.SweepExpired(context.Background()); err != nil {
			t.Fatalf("sweep the questions nobody answered: %v", err)
		}
	}

	if len(h.stranded) != 1 {
		t.Fatalf("the sweep stopped the same run %d times, want once", len(h.stranded))
	}
}

func TestAQuestionNobodyCouldAnswerIsRefusedWhenTheRunAsksIt(t *testing.T) {
	h := newHarness(t)
	ctx := h.asRunner(context.Background())

	unanswerable := channelQuestion("01REF", true)
	unanswerable.Options = nil
	unanswerable.AllowFreeText = false

	err := h.service.Asked(ctx, h.runner, h.message("01REF", unanswerable))

	var refused entity.ValidationError
	if !errors.As(err, &refused) {
		t.Fatalf(
			"an agent stopped for a question with no options and no free text, and it was taken; "+
				"the run would park on something nobody can settle. Answered %v",
			err,
		)
	}
}
