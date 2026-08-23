package execution_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	channelv1 "github.com/usenorn/norn/pkg/channel/v1"
)

func answered(question string, answer string) entity.IssueQuestion {
	at := time.Now().UTC()
	settler := uuid.New()

	return entity.IssueQuestion{
		ID:             uuid.New(),
		ExecutionID:    "exec-01ABC",
		Ref:            "01QUESTION",
		Kind:           entity.QuestionDecision,
		Blocking:       true,
		State:          entity.QuestionAnswered,
		Question:       question,
		Answer:         answer,
		AnsweredBy:     settler,
		AnsweredByName: "Rae",
		AnsweredAt:     &at,
		SettledBy:      settler,
		SettledByName:  "Rae",
		SettledAt:      &at,
	}
}

func TestAnsweringARunThatIsStillWorkingSendsTheAnswerWithoutMovingIt(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionRunning))

	question := answered("Keep the old endpoint?", "Keep for 30 days")

	if err := h.service.Answered(context.Background(), question); err != nil {
		t.Fatalf("hand a running agent its answer: %v", err)
	}

	sent, delivered := h.sent(entity.ChannelQuestionAnswered)
	if !delivered {
		t.Fatal("the agent was left holding the question open with the answer already recorded")
	}

	var payload channelv1.Answer
	if err := json.Unmarshal(sent.Payload, &payload); err != nil {
		t.Fatalf("read what went down the channel: %v", err)
	}

	if payload.Answer != question.Answer || payload.QuestionID != question.ID.String() {
		t.Fatalf(
			"the machine was sent %+v, want the answer verbatim against question %s",
			payload, question.ID,
		)
	}

	if _, moved := h.sent(entity.ChannelExecutionResume); moved {
		t.Fatal(
			"a run that never stopped was told to resume; the session would be restarted under an " +
				"agent that is still mid-turn",
		)
	}
}

func TestAnsweringAParkedRunResumesTheSameSessionWithTheAnswerAttached(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionWaitingForInput))
	h.moving()

	question := answered("Keep the old endpoint?", "Remove it now")

	if err := h.service.Answered(context.Background(), question); err != nil {
		t.Fatalf("answer a parked run: %v", err)
	}

	sent, resumed := h.sent(entity.ChannelExecutionResume)
	if !resumed {
		t.Fatal("a parked run was answered and never told to carry on")
	}

	var payload channelv1.Instruction
	if err := json.Unmarshal(sent.Payload, &payload); err != nil {
		t.Fatalf("read the resume: %v", err)
	}

	switch {
	case payload.Reason != channelv1.ResumeAnswer:
		t.Fatalf("the resume said %q, want %q", payload.Reason, channelv1.ResumeAnswer)
	case payload.Instruction != question.Answer:
		t.Fatalf("the resume carried %q, want the answer verbatim", payload.Instruction)
	case payload.QuestionID != question.ID.String():
		t.Fatalf(
			"the resume named question %q; without the id the agent cannot tell which of its "+
				"questions this answers",
			payload.QuestionID,
		)
	}

	moved, ok := h.moved(entity.ExecutionQueuedForResume)
	if !ok {
		t.Fatal("the run stayed parked after somebody answered it")
	}

	if moved.Reason == "" || moved.Actor.AccountID != question.SettledBy {
		t.Fatalf(
			"the timeline recorded %+v; a reader cannot see who answered or what they said",
			moved,
		)
	}
}

func TestAnsweringARunThatHasAlreadyFinishedSendsItNothing(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionCompleted))

	if err := h.service.Answered(context.Background(), answered("Which one?", "The first")); err != nil {
		t.Fatalf("answer a finished run: %v", err)
	}

	if len(h.spooled) != 0 {
		t.Fatalf(
			"%d messages went to a machine that finished this run; the answer is recorded and "+
				"there is nobody left to hand it to",
			len(h.spooled),
		)
	}
}

func TestAQuestionNobodyWillAnswerStopsTheRunItParked(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionWaitingForInput))
	h.moving()

	question := answered("Which one?", "")
	question.State = entity.QuestionExpired

	const reason = "nobody answered the question this run stopped on"

	if err := h.service.Unanswerable(context.Background(), question, reason); err != nil {
		t.Fatalf("stop a run nobody answered: %v", err)
	}

	moved, ok := h.moved(entity.ExecutionFailed)
	if !ok {
		t.Fatal(
			"the run is still parked on a question that will never be answered, holding its lease " +
				"and explaining nothing to whoever finds it",
		)
	}

	if moved.Reason != reason {
		t.Fatalf("the run failed saying %q, want %q", moved.Reason, reason)
	}
}

func TestARunThatCarriedOnIsNotStoppedByItsQuestionRunningOut(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionRunning))

	question := answered("Which one?", "")
	question.Blocking = false
	question.State = entity.QuestionExpired

	if err := h.service.Unanswerable(context.Background(), question, "ran out of time"); err != nil {
		t.Fatalf("settle a question the run did not stop for: %v", err)
	}

	if _, failed := h.moved(entity.ExecutionFailed); failed {
		t.Fatal(
			"a working run was killed because a question it had already carried on past ran out " +
				"of time",
		)
	}
}

func TestAQuestionARunAskedIsOnThatRunsOwnTimeline(t *testing.T) {
	h := newHarness(t)
	h.holding(h.execution(entity.ExecutionRunning))

	question := answered("Keep the old endpoint?", "")
	question.State = entity.QuestionAsked
	question.CreatedAt = time.Now().UTC()

	if err := h.service.Questioned(context.Background(), question); err != nil {
		t.Fatalf("record what the run asked: %v", err)
	}

	entry, ok := h.entry(entity.ExecutionEventQuestion)
	if !ok {
		t.Fatal(
			"nothing on the run's timeline says it asked anything, so a reader sees a run that " +
				"went quiet for no reason",
		)
	}

	if entry.Reason != question.Question {
		t.Fatalf("the timeline says %q, want the question as asked", entry.Reason)
	}
}
