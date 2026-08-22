package issuequestion_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/repository"
	activityrepo "github.com/usenorn/norn/internal/repository/activity"
	issuerepo "github.com/usenorn/norn/internal/repository/issue"
	delegationrepo "github.com/usenorn/norn/internal/repository/issuedelegation"
	questionrepo "github.com/usenorn/norn/internal/repository/issuequestion"
	notifyrepo "github.com/usenorn/norn/internal/repository/notificationevent"
	transactorrepo "github.com/usenorn/norn/internal/repository/transactor"
	"github.com/usenorn/norn/internal/service"
	authorizersvc "github.com/usenorn/norn/internal/service/authorizer"
	eventsvc "github.com/usenorn/norn/internal/service/event"
	executionsvc "github.com/usenorn/norn/internal/service/execution"
	questionsvc "github.com/usenorn/norn/internal/service/issuequestion"
)

type harness struct {
	questions   *questionrepo.MockIssueQuestion
	issues      *issuerepo.MockIssue
	delegations *delegationrepo.MockIssueDelegation
	activity    *activityrepo.MockActivity
	notify      *notifyrepo.MockNotificationEvent
	executions  *executionsvc.MockExecutions
	events      *eventsvc.MockEvents
	authorizer  *authorizersvc.MockAuthorizer
	service     service.IssueQuestions

	workspaceID uuid.UUID
	issue       entity.Issue
	runner      entity.Runner
	execution   entity.Execution
	caller      uuid.UUID

	stored   []entity.IssueQuestion
	recorded []entity.Activity
	answered []entity.IssueQuestion
	stranded []entity.IssueQuestion
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	ctrl := gomock.NewController(t)

	workspaceID := uuid.New()
	teamID := uuid.New()
	agentID := uuid.New()

	h := &harness{
		questions:   questionrepo.NewMockIssueQuestion(ctrl),
		issues:      issuerepo.NewMockIssue(ctrl),
		delegations: delegationrepo.NewMockIssueDelegation(ctrl),
		activity:    activityrepo.NewMockActivity(ctrl),
		notify:      notifyrepo.NewMockNotificationEvent(ctrl),
		executions:  executionsvc.NewMockExecutions(ctrl),
		events:      eventsvc.NewMockEvents(ctrl),
		authorizer:  authorizersvc.NewMockAuthorizer(ctrl),
		workspaceID: workspaceID,
		caller:      uuid.New(),
		issue: entity.Issue{
			ID:           uuid.New(),
			WorkspaceID:  workspaceID,
			TeamID:       teamID,
			ReferenceKey: "NORN",
			Number:       37,
			Title:        "Questions from an execution",
		},
		runner: entity.Runner{
			ID:          uuid.New(),
			WorkspaceID: workspaceID,
			AgentID:     agentID,
			Name:        "vlad-mbp",
			Status:      entity.RunnerStatusActive,
		},
	}

	h.execution = entity.Execution{
		ID:          "exec-01ABC",
		WorkspaceID: workspaceID,
		IssueID:     h.issue.ID,
		TeamID:      teamID,
		AgentID:     agentID,
		RunnerID:    h.runner.ID,
		State:       entity.ExecutionRunning,
	}

	transactor := transactorrepo.NewMockTransactor(ctrl)
	transactor.EXPECT().
		WithTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).
		AnyTimes()

	h.expectStore()
	h.expectSurroundings()

	h.service = questionsvc.New(
		h.questions, h.issues, h.delegations, h.activity, h.notify,
		h.executions, h.events, transactor, h.authorizer,
	)

	return h
}

// expectStore keeps the rows the service writes in memory and enforces the two rules the schema
// declares: one question per ref per run, and a settled question that nothing settles twice.
func (h *harness) expectStore() {
	h.questions.EXPECT().
		Ask(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, question entity.IssueQuestion) (entity.IssueQuestion, error) {
			for _, held := range h.stored {
				if held.Ref != "" && held.Ref == question.Ref &&
					held.ExecutionID == question.ExecutionID {
					return entity.IssueQuestion{}, entity.ErrIssueQuestionRecorded
				}
			}

			question.ID = uuid.New()
			question.State = entity.QuestionAsked
			question.CreatedAt = time.Now().UTC()
			h.stored = append(h.stored, question)

			return question, nil
		}).
		AnyTimes()

	h.questions.EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, id uuid.UUID) (entity.IssueQuestion, error) {
			for _, held := range h.stored {
				if held.ID == id {
					return held, nil
				}
			}

			return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
		}).
		AnyTimes()

	h.questions.EXPECT().
		Answer(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, answer repository.QuestionAnswer,
		) (entity.IssueQuestion, error) {
			for index, held := range h.stored {
				if held.ID != answer.QuestionID {
					continue
				}

				if held.Settled() {
					return entity.IssueQuestion{}, entity.ErrIssueQuestionAnswered
				}

				held.Answer = answer.Answer
				held.AnsweredBy = answer.AccountID
				held.AnsweredAt = &answer.AnsweredAt
				held.SettledAt = &answer.AnsweredAt
				held.SettledBy = answer.AccountID
				held.State = entity.QuestionAnswered
				h.stored[index] = held

				return held, nil
			}

			return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
		}).
		AnyTimes()

	h.questions.EXPECT().
		Settle(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ uuid.UUID, settlement repository.QuestionSettlement,
		) (entity.IssueQuestion, error) {
			for index, held := range h.stored {
				if held.ID != settlement.QuestionID {
					continue
				}

				if held.Settled() {
					return entity.IssueQuestion{}, entity.ErrIssueQuestionSettled
				}

				held.State = settlement.State
				held.SettledBy = settlement.AccountID
				held.SettledAt = &settlement.SettledAt
				h.stored[index] = held

				return held, nil
			}

			return entity.IssueQuestion{}, entity.ErrIssueQuestionNotFound
		}).
		AnyTimes()

	h.questions.EXPECT().
		Lapsed(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, now time.Time, _ int) ([]entity.IssueQuestion, error) {
			lapsed := make([]entity.IssueQuestion, 0)

			for _, held := range h.stored {
				if !held.Settled() && held.Deadline.Before(now) {
					lapsed = append(lapsed, held)
				}
			}

			return lapsed, nil
		}).
		AnyTimes()
}

func (h *harness) expectSurroundings() {
	h.activity.EXPECT().
		Record(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, entry entity.Activity) error {
			h.recorded = append(h.recorded, entry)

			return nil
		}).
		AnyTimes()

	h.notify.EXPECT().Record(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	h.events.EXPECT().Publish(gomock.Any(), gomock.Any()).AnyTimes()

	h.delegations.EXPECT().
		Open(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entity.IssueDelegation{DelegatedByAccountID: h.caller}, nil).
		AnyTimes()

	h.issues.EXPECT().
		GetVisible(gomock.Any(), gomock.Any(), h.issue.ID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ uuid.UUID, _ entity.TeamScope) (entity.Issue, error) {
			return h.issue, nil
		}).
		AnyTimes()

	h.authorizer.EXPECT().
		Decide(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request entity.AccessRequest) (entity.Decision, error) {
			return entity.Decision{
				Actor: entity.Actor{Kind: entity.ActorKindUser, AccountID: h.caller},
				Role:  entity.MembershipRoleAdmin,
				Scope: entity.TeamScope{WorkspaceID: request.WorkspaceID, AllTeams: true},
			}, nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		Held(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, runner entity.Runner, executionID string,
		) (entity.Execution, error) {
			if runner.ID != h.execution.RunnerID || executionID != h.execution.ID {
				return entity.Execution{}, entity.ErrExecutionNotFound
			}

			return h.execution, nil
		}).
		AnyTimes()

	h.executions.EXPECT().Questioned(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	h.executions.EXPECT().
		Answered(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, question entity.IssueQuestion) error {
			h.answered = append(h.answered, question)

			return nil
		}).
		AnyTimes()

	h.executions.EXPECT().
		Unanswerable(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, question entity.IssueQuestion, _ string) error {
			h.stranded = append(h.stranded, question)

			return nil
		}).
		AnyTimes()
}

func (h *harness) asking(ref string, blocking bool) entity.ChannelMessage {
	return h.message(ref, channelQuestion(ref, blocking))
}

func (h *harness) message(id string, payload any) entity.ChannelMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	return entity.ChannelMessage{
		ID:          id,
		Type:        entity.ChannelQuestionAsked,
		ExecutionID: h.execution.ID,
		IssuedAt:    time.Now().UTC(),
		Payload:     body,
	}
}

func (h *harness) only(t *testing.T) entity.IssueQuestion {
	t.Helper()

	if len(h.stored) != 1 {
		t.Fatalf("the issue holds %d questions, want exactly 1", len(h.stored))
	}

	return h.stored[0]
}

func (h *harness) noted(kind entity.ActivityKind) (entity.Activity, bool) {
	for _, entry := range h.recorded {
		if entry.Kind == kind {
			return entry, true
		}
	}

	return entity.Activity{}, false
}
