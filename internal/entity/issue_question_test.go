package entity_test

import (
	"testing"
	"time"

	"github.com/usenorn/norn/internal/entity"
)

func TestOnlyTheKindsAndStatesTheSchemaAllowsAreValid(t *testing.T) {
	for _, kind := range entity.QuestionKinds() {
		if !kind.Valid() {
			t.Errorf("%q is offered as a kind but does not pass its own check", kind)
		}
	}

	if entity.QuestionKind("permission").Valid() {
		t.Error("a kind the table's check constraint would refuse passed validation")
	}

	for _, state := range entity.QuestionStates() {
		if !state.Valid() {
			t.Errorf("%q is offered as a state but does not pass its own check", state)
		}
	}

	if entity.QuestionState("pending").Valid() {
		t.Error("a state the table's check constraint would refuse passed validation")
	}
}

func TestOnlyAnUnsettledQuestionIsStillWaitingOnSomebody(t *testing.T) {
	if entity.QuestionAsked.Settled() {
		t.Error("a question nobody has dealt with reads as settled, so nothing will chase it")
	}

	for _, state := range []entity.QuestionState{
		entity.QuestionAnswered, entity.QuestionDismissed, entity.QuestionExpired,
	} {
		if !state.Settled() {
			t.Errorf("%q does not read as settled, so the sweep would deal with it a second time", state)
		}
	}
}

func TestAQuestionReadsAsExpiredFromTheClockBeforeTheSweepGetsToIt(t *testing.T) {
	now := time.Now().UTC()

	lapsed := entity.IssueQuestion{State: entity.QuestionAsked, Deadline: now.Add(-time.Minute)}
	if !lapsed.Expired(now) {
		t.Error(
			"a question past its deadline is shown as still waiting until a scheduled sweep " +
				"catches up, so a person is told an agent is waiting on them when it is not",
		)
	}

	standing := entity.IssueQuestion{State: entity.QuestionAsked, Deadline: now.Add(time.Hour)}
	if standing.Expired(now) {
		t.Error("a question still inside its deadline reads as expired")
	}

	at := now.Add(-time.Hour)
	answered := entity.IssueQuestion{
		State:      entity.QuestionAnswered,
		Deadline:   now.Add(-time.Minute),
		Answer:     "Remove now",
		AnsweredAt: &at,
		SettledAt:  &at,
	}

	if answered.Expired(now) {
		t.Error("a question somebody answered before the deadline reads as expired afterwards")
	}
}

func TestOnlyARunThatStoppedAndIsStillWaitingCountsAsParked(t *testing.T) {
	parked := entity.IssueQuestion{
		Blocking: true, ExecutionID: "exec-01ABC", State: entity.QuestionAsked,
	}
	if !parked.Parked() {
		t.Error("a run that stopped for an unanswered question does not read as parked")
	}

	for name, question := range map[string]entity.IssueQuestion{
		"one the agent carried on past": {
			Blocking: false, ExecutionID: "exec-01ABC", State: entity.QuestionAsked,
		},
		"one no run is behind": {
			Blocking: true, State: entity.QuestionAsked,
		},
		"one somebody already answered": {
			Blocking: true, ExecutionID: "exec-01ABC", State: entity.QuestionAnswered,
		},
	} {
		if question.Parked() {
			t.Errorf("%s reads as parked, so settling it would stop a run that is not waiting", name)
		}
	}
}

func TestAClosedSetOfOptionsOnlyTakesOneOfThem(t *testing.T) {
	closed := entity.IssueQuestion{
		Options:       []string{"Keep for 30 days", "Remove now"},
		AllowFreeText: false,
	}

	if !closed.Acceptable("  Remove now  ") {
		t.Error("an offered option was refused because of the whitespace around it")
	}

	if closed.Acceptable("Do something else") {
		t.Error(
			"free text was taken against a question that offered a closed set, so the agent gets " +
				"an answer it has no branch for",
		)
	}

	open := entity.IssueQuestion{
		Options:       []string{"Keep for 30 days", "Remove now"},
		AllowFreeText: true,
	}

	if !open.Acceptable("Keep it behind a flag") {
		t.Error("free text was refused by a question that said it would take some")
	}

	if !(entity.IssueQuestion{AllowFreeText: true}).Acceptable("Anything at all") {
		t.Error("an open question refused an answer")
	}
}

func TestAQuestionNobodyCouldAnswerIsRefused(t *testing.T) {
	if (entity.ValidateQuestionReachable("options", nil, false) == entity.FieldError{}) {
		t.Error(
			"a question with no options and no free text passed validation; a run parking on one " +
				"waits for something a person has no way to give it",
		)
	}

	if (entity.ValidateQuestionReachable("options", nil, true) != entity.FieldError{}) {
		t.Error("an open question was refused")
	}

	if (entity.ValidateQuestionReachable("options", []string{"Yes"}, false) != entity.FieldError{}) {
		t.Error("a question offering one option to pick was refused")
	}
}

func TestMoreOptionsThanTheTableTakesAreRefusedBeforeItSeesThem(t *testing.T) {
	options := make([]string, entity.QuestionOptionsMax+1)
	for index := range options {
		options[index] = "an answer"
	}

	if (entity.ValidateQuestionOptions("options", options) == entity.FieldError{}) {
		t.Errorf(
			"%d options passed validation; the table takes %d and would refuse the row with "+
				"nothing naming which limit did it",
			len(options), entity.QuestionOptionsMax,
		)
	}

	if (entity.ValidateQuestionOptions("options", []string{"Yes", "  "}) == entity.FieldError{}) {
		t.Error("an option with nothing in it passed validation, so a person is offered a blank button")
	}
}
